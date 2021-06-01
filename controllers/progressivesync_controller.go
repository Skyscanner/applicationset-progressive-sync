/**
 * Copyright 2021 Skyscanner Limited.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/scheduler"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/utils"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ProgressiveSyncReconciler reconciles a ProgressiveSync object
type ProgressiveSyncReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	ArgoCDAppClient utils.ArgoCDAppClient
}

// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications/status,verbs=get;list;watch

// Reconcile performs the reconciling for a single named ProgressiveSync object
func (r *ProgressiveSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("ProgressiveSync", req.NamespacedName)

	// Get the ProgressiveSync object
	pr := syncv1alpha1.ProgressiveSync{}
	if err := r.Get(ctx, req.NamespacedName, &pr); err != nil {
		log.Error(err, "unable to fetch ProgressiveSync")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log = r.Log.WithValues("applicationset", pr.Spec.SourceRef.Name)

	if pr.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object
		if !controllerutil.ContainsFinalizer(&pr, syncv1alpha1.ProgressiveSyncFinalizer) {
			controllerutil.AddFinalizer(&pr, syncv1alpha1.ProgressiveSyncFinalizer)
			if err := r.Update(ctx, &pr); err != nil {
				r.Log.Error(err, "failed to update object")
				return ctrl.Result{}, err
			}
			// Requeue after adding the finalizer
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(&pr, syncv1alpha1.ProgressiveSyncFinalizer) {
			// Remove our finalizer from the list and update it
			controllerutil.RemoveFinalizer(&pr, syncv1alpha1.ProgressiveSyncFinalizer)
			if err := r.Update(ctx, &pr); err != nil {
				r.Log.Error(err, "failed to update object")
				return ctrl.Result{}, err
			}
		}
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	for _, stage := range pr.Spec.Stages {
		log = r.Log.WithValues("stage", stage.Name)

		// Get the clusters to update
		clusters, err := r.getClustersFromSelector(stage.Targets.Clusters.Selector)
		if err != nil {
			message := "unable to fetch clusters"
			log.Error(err, message)
			if err := r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseUnknown, &pr); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, err
		}
		r.Log.V(1).Info("clusters selected", "clusters", utils.GetClustersName(clusters.Items))

		// Get only the Applications owned by the ProgressiveSync targeting the selected clusters
		apps, err := r.getOwnedAppsFromClusters(clusters, &pr)
		if err != nil {
			message := "unable to fetch apps"
			log.Error(err, message)
			if err := r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseUnknown, &pr); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, err
		}
		r.Log.V(1).Info("apps selected", "apps", utils.GetAppsName(apps))

		// Remove the annotation from the OutOfSync Applications before passing them to the Scheduler
		// This action allows the Scheduler to keep track at which stage an Application has been synced.
		outOfSyncApps := utils.GetAppsBySyncStatusCode(apps, argov1alpha1.SyncStatusCodeOutOfSync)
		if err := r.removeAnnotationFromApps(outOfSyncApps, utils.ProgressiveSyncSyncedAtStageKey); err != nil {
			message := "unable to remove out-of-sync annotation"
			log.Error(err, message)
			if err := r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseUnknown, &pr); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, err
		}

		// Get the Applications to update
		// TODO: update the scheduler to return the InProgress apps
		scheduledApps := scheduler.Scheduler(apps, stage)

		// TODO: status status - update targets

		for _, s := range scheduledApps {
			r.Log.Info("syncing app", "app", s)

			_, err := r.syncApp(s.Name)

			if err != nil {
				if !strings.Contains(err.Error(), "another operation is already in progress") {
					message := "unable to sync app"
					log.Error(err, message, "message", err.Error())
					if err := r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseUnknown, &pr); err != nil {
						return ctrl.Result{}, err
					}

					// TODO: stage status - update failed clusters

					return ctrl.Result{}, err
				}
			}
		}

		if scheduler.IsStageFailed(apps) {
			message := "stage failed"
			r.Log.Info(message)
			if err := r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &pr); err != nil {
				return ctrl.Result{}, err
			}

			failedMessage := fmt.Sprintf("%s stage failed", stage.Name)
			failed := pr.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesFailedReason, failedMessage)
			apimeta.SetStatusCondition(pr.GetStatusConditions(), failed)
			if err := r.patchStatus(ctx, &pr); err != nil {
				r.Log.Error(err, "failed to update object status")
				return ctrl.Result{}, err
			}
			r.Log.Info("rollout failed")
			// We can set Requeue: true once we have a timeout in place
			return ctrl.Result{}, nil
		}

		if scheduler.IsStageInProgress(apps) {
			message := "stage in progress"
			r.Log.Info(message)
			if err := r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseProgressing, &pr); err != nil {
				return ctrl.Result{}, err
			}
			// Stage in progress, we reconcile again until the stage is completed or failed
			return ctrl.Result{Requeue: true}, nil
		}

		if scheduler.IsStageComplete(apps) {
			message := "stage completed"
			r.Log.Info(message)
			if err := r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseSucceeded, &pr); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			message := "stage is unknown"
			r.Log.Info(message)
			if err := r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseUnknown, &pr); err != nil {
				return ctrl.Result{}, err
			}
			// We can set Requeue: true once we have a timeout in place
			return ctrl.Result{}, nil
		}
	}

	log.Info("all stages completed")

	// Progressive rollout completed
	completed := pr.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesCompleteReason, "All stages completed")
	apimeta.SetStatusCondition(pr.GetStatusConditions(), completed)
	if err := r.patchStatus(ctx, &pr); err != nil {
		r.Log.Error(err, "failed to update object status")
		return ctrl.Result{}, err
	}
	r.Log.Info("rollout completed")
	return ctrl.Result{}, nil
}

// SetupWithManager adds the reconciler to the manager, so that it gets started when the manager is started.
func (r *ProgressiveSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncv1alpha1.ProgressiveSync{}).
		Watches(
			&source.Kind{Type: &argov1alpha1.Application{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForApplicationChange)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(r.requestsForSecretChange),
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Complete(r)
}

// requestsForApplicationChange returns a reconcile request for a Progressive Rollout object when an Application change
func (r *ProgressiveSyncReconciler) requestsForApplicationChange(o client.Object) []reconcile.Request {

	/*
		We trigger a Progressive Rollout reconciliation loop on an Application event if:
		- the Application owner is referenced by a ProgressiveSync object
	*/

	var requests []reconcile.Request
	var list syncv1alpha1.ProgressiveSyncList
	ctx := context.Background()

	app, ok := o.(*argov1alpha1.Application)
	if !ok {
		err := fmt.Errorf("expected application, got %T", o)
		r.Log.Error(err, "failed to convert object to application")
		return nil
	}

	if err := r.List(ctx, &list); err != nil {
		r.Log.Error(err, "failed to list ProgressiveSync")
		return nil
	}

	for _, pr := range list.Items {
		if pr.Owns(app.GetOwnerReferences()) {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pr.Namespace,
				Name:      pr.Name,
			}})
		}
	}

	return requests
}

// requestsForSecretChange returns a reconcile request for a Progressive Rollout object when a Secret change
func (r *ProgressiveSyncReconciler) requestsForSecretChange(o client.Object) []reconcile.Request {

	/*
		We trigger a Progressive Rollout reconciliation loop on a Secret event if:
		- the Secret is an ArgoCD cluster, AND
		- there is an Application targeting that secret/cluster, AND
		- that Application owner is referenced by a ProgressiveSync object
	*/

	var requests []reconcile.Request
	var prList syncv1alpha1.ProgressiveSyncList
	var appList argov1alpha1.ApplicationList
	requestsMap := make(map[types.NamespacedName]bool)
	ctx := context.Background()

	s, ok := o.(*corev1.Secret)
	if !ok {
		err := fmt.Errorf("expected secret, got %T", o)
		r.Log.Error(err, "failed to convert object to secret")
		return nil
	}

	r.Log.V(1).Info("received secret event", "name", s.Name, "Namespace", s.Namespace)

	if !utils.IsArgoCDCluster(s.GetLabels()) {
		return nil
	}

	if err := r.List(ctx, &prList); err != nil {
		r.Log.Error(err, "failed to list ProgressiveSync")
		return nil
	}
	if err := r.List(ctx, &appList); err != nil {
		r.Log.Error(err, "failed to list Application")
		return nil
	}

	for _, pr := range prList.Items {
		for _, app := range appList.Items {
			if app.Spec.Destination.Server == string(s.Data["server"]) && pr.Owns(app.GetOwnerReferences()) {
				/*
					Consider the following scenario:
					- 2 Applications
					- owned by the same ApplicationSet
					- referenced by the same Progressive Rollout
					- targeting the same cluster

					In this scenario, we would trigger the reconciliation loop twice.
					To avoid that, we use a map to store for which Progressive Rollout objects we already trigger the reconciliation loop.
				*/

				namespacedName := types.NamespacedName{Name: pr.Name, Namespace: pr.Namespace}
				if _, ok := requestsMap[namespacedName]; !ok {
					requestsMap[namespacedName] = true
					requests = append(requests, reconcile.Request{NamespacedName: namespacedName})
				}
			}
		}
	}

	return requests
}

// getClustersFromSelector returns a list of ArgoCD clusters matching the provided label selector
func (r *ProgressiveSyncReconciler) getClustersFromSelector(selector metav1.LabelSelector) (corev1.SecretList, error) {
	secrets := corev1.SecretList{}
	ctx := context.Background()

	argoSelector := metav1.AddLabelToSelector(&selector, utils.ArgoCDSecretTypeLabel, utils.ArgoCDSecretTypeCluster)
	labels, err := metav1.LabelSelectorAsSelector(argoSelector)
	if err != nil {
		r.Log.Error(err, "unable to convert selector into labels")
		return corev1.SecretList{}, err
	}

	if err = r.List(ctx, &secrets, client.MatchingLabelsSelector{Selector: labels}); err != nil {
		r.Log.Error(err, "failed to select targets using labels selector")
		return corev1.SecretList{}, err
	}

	// https://github.com/Skyscanner/applicationset-progressive-sync/issues/9 will provide a better sorting
	utils.SortSecretsByName(&secrets)

	return secrets, nil
}

// getOwnedAppsFromClusters returns a list of Applications targeting the specified clusters and owned by the specified ProgressiveSync
func (r *ProgressiveSyncReconciler) getOwnedAppsFromClusters(clusters corev1.SecretList, pr *syncv1alpha1.ProgressiveSync) ([]argov1alpha1.Application, error) {
	var apps []argov1alpha1.Application
	appList := argov1alpha1.ApplicationList{}
	ctx := context.Background()

	if err := r.List(ctx, &appList); err != nil {
		r.Log.Error(err, "failed to list Application")
		return apps, err
	}

	for _, c := range clusters.Items {
		for _, app := range appList.Items {
			if pr.Owns(app.GetOwnerReferences()) && string(c.Data["server"]) == app.Spec.Destination.Server {
				apps = append(apps, app)
			}
		}
	}

	utils.SortAppsByName(apps)

	return apps, nil
}

// removeAnnotationFromApps remove an annotation from the given Applications
func (r *ProgressiveSyncReconciler) removeAnnotationFromApps(apps []argov1alpha1.Application, annotation string) error {
	ctx := context.Background()

	for _, app := range apps {
		if _, ok := app.Annotations[annotation]; ok {
			delete(app.Annotations, annotation)
			if err := r.Client.Update(ctx, &app); err != nil {
				r.Log.Error(err, "failed to update Application", "app", app.Name)
				return err
			}
		}
	}
	return nil
}

// updateStageStatus updates the target stage given a stage name, message and phase
func (r *ProgressiveSyncReconciler) updateStageStatus(ctx context.Context, name, message string, phase syncv1alpha1.StageStatusPhase, pr *syncv1alpha1.ProgressiveSync) error {
	stageStatus := syncv1alpha1.NewStageStatus(
		name,
		message,
		phase,
	)
	nowTime := metav1.NewTime(time.Now())
	pr.SetStageStatus(stageStatus, &nowTime)
	if err := r.patchStatus(ctx, pr); err != nil {
		r.Log.Error(err, "failed to update object status")
		return err
	}
	return nil
}

// syncApp sends a sync request for the target app
func (r *ProgressiveSyncReconciler) syncApp(appName string) (*argov1alpha1.Application, error) {
	ctx := context.Background()

	syncReq := applicationpkg.ApplicationSyncRequest{
		Name: &appName,
	}

	return r.ArgoCDAppClient.Sync(ctx, &syncReq)
}

// patchStatus patches the progressive sync object status
func (r *ProgressiveSyncReconciler) patchStatus(ctx context.Context, pr *syncv1alpha1.ProgressiveSync) error {
	key := client.ObjectKeyFromObject(pr)
	latest := syncv1alpha1.ProgressiveSync{}
	if err := r.Client.Get(ctx, key, &latest); err != nil {
		return err
	}
	return r.Client.Status().Patch(ctx, pr, client.MergeFrom(&latest))
}
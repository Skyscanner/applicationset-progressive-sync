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
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
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
	Log                logr.Logger
	Scheme             *runtime.Scheme
	ArgoCDAppClient    utils.ArgoCDAppClient
	SyncedAtStage      map[string]string // Key: App name, Value: Stage name
	SyncedAppsPerStage map[string]int    // Key: Stage name, Value: Number of apps per stage
}

// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications/status,verbs=get;list;watch

// Reconcile performs the reconciling for a single named ProgressiveSync object
func (r *ProgressiveSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("progressivesync", req.NamespacedName)
	log.Info("reconciliation loop started")

	// Get the ProgressiveSync object
	var ps syncv1alpha1.ProgressiveSync
	if err := r.Get(ctx, req.NamespacedName, &ps); err != nil {
		log.Error(err, "unable to fetch progressivesync object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log = log.WithValues("applicationset", ps.Spec.SourceRef.Name)

	// If the object is being deleted, remove finalizer and don't requeue it
	if !ps.ObjectMeta.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(&ps, syncv1alpha1.ProgressiveSyncFinalizer)
		if err := r.Update(ctx, &ps); err != nil {
			log.Error(err, "failed to update object when removing finalizer")
			return ctrl.Result{}, err
		}
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&ps, syncv1alpha1.ProgressiveSyncFinalizer) {
		controllerutil.AddFinalizer(&ps, syncv1alpha1.ProgressiveSyncFinalizer)
		if err := r.Update(ctx, &ps); err != nil {
			log.Error(err, "failed to update object when adding finalizer")
			return ctrl.Result{}, err
		}
		// Requeue after adding the finalizer
		return ctrl.Result{Requeue: true}, nil
	}

	latest := ps
	var result reconcile.Result
	var reconcileErr error

	for _, stage := range ps.Spec.Stages {
		log = log.WithValues("stage", stage.Name)

		latest, result, reconcileErr = r.reconcileStage(ctx, latest, stage)
		if err := r.updateStatusWithRetry(ctx, &latest); err != nil {
			return ctrl.Result{}, err
		}

		if result.Requeue || reconcileErr != nil {
			log.Info("requeuing stage")
			return result, reconcileErr
		}

	}

	// Progressive sync completed
	completed := latest.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesCompleteReason, "All stages completed")
	apimeta.SetStatusCondition(latest.GetStatusConditions(), completed)
	if err := r.updateStatusWithRetry(ctx, &latest); err != nil {
		log.Error(err, "failed to update object status")
		return ctrl.Result{}, err
	}
	log.Info("sync completed")
	return ctrl.Result{}, nil
}

// SetupWithManager adds the reconciler to the manager, so that it gets started when the manager is started.
func (r *ProgressiveSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncv1alpha1.ProgressiveSync{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&source.Kind{Type: &argov1alpha1.Application{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForApplicationChange)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(r.requestsForSecretChange),
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Complete(r)
}

// requestsForApplicationChange returns a reconcile request when an Application changes
func (r *ProgressiveSyncReconciler) requestsForApplicationChange(o client.Object) []reconcile.Request {

	/*
		We trigger a reconciliation loop on an Application event if:
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

// requestsForSecretChange returns a reconcile request when a Secret changes
func (r *ProgressiveSyncReconciler) requestsForSecretChange(o client.Object) []reconcile.Request {

	/*
		We trigger a reconciliation loop on a Secret event if:
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

	r.Log.Info("received secret event", "name", s.Name, "Namespace", s.Namespace)

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
						- two Applications
						- owned by the same ApplicationSet
						- referenced by the same ProgressiveSync
						- targeting the same cluster

						In this scenario, we would trigger the reconciliation loop twice.
						To avoid that, we use a map to store for which ProgressiveSync object
					    we already triggered the reconciliation loop.
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
func (r *ProgressiveSyncReconciler) getClustersFromSelector(ctx context.Context, selector metav1.LabelSelector) (corev1.SecretList, error) {
	secrets := corev1.SecretList{}

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
func (r *ProgressiveSyncReconciler) getOwnedAppsFromClusters(ctx context.Context, clusters corev1.SecretList, pr *syncv1alpha1.ProgressiveSync) ([]argov1alpha1.Application, error) {
	var apps []argov1alpha1.Application
	appList := argov1alpha1.ApplicationList{}

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

// updateStageStatus updates the target stage given a stage name, message and phase
func (r *ProgressiveSyncReconciler) updateStageStatus(ctx context.Context, name, message string, phase syncv1alpha1.StageStatusPhase, pr *syncv1alpha1.ProgressiveSync) {
	stageStatus := syncv1alpha1.NewStageStatus(
		name,
		message,
		phase,
	)
	nowTime := metav1.NewTime(time.Now())
	pr.SetStageStatus(stageStatus, &nowTime)
}

// syncApp sends a sync request for the target app
func (r *ProgressiveSyncReconciler) syncApp(ctx context.Context, appName string) (*argov1alpha1.Application, error) {
	syncReq := applicationpkg.ApplicationSyncRequest{
		Name: &appName,
	}

	return r.ArgoCDAppClient.Sync(ctx, &syncReq)
}

// updateStatusWithRetry updates the progressive sync object status with backoff
func (r *ProgressiveSyncReconciler) updateStatusWithRetry(ctx context.Context, pr *syncv1alpha1.ProgressiveSync) error {

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {

		key := client.ObjectKeyFromObject(pr)
		latest := syncv1alpha1.ProgressiveSync{}
		if err := r.Client.Get(ctx, key, &latest); err != nil {
			return err
		}
		latest.Status = pr.Status
		if err := r.Client.Status().Update(ctx, &latest); err != nil {
			return err
		}
		return nil

	})
	return retryErr
}

// setSyncedAtAnnotation sets the SyncedAt annotation for the currently progressing stage of the app
func (r *ProgressiveSyncReconciler) setSyncedAtAnnotation(ctx context.Context, app argov1alpha1.Application, stageName string, stageMaxTargets int, ps syncv1alpha1.ProgressiveSync) error {

	log := r.Log.WithValues("stage", stageName)

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		key := client.ObjectKeyFromObject(&app)
		latest := argov1alpha1.Application{}

		if err := r.Client.Get(ctx, key, &latest); err != nil {
			log.Info("failed to get app when adding syncedAt annotation")
			return err
		}

		log = log.WithValues("app", fmt.Sprintf("%s/%s", app.Namespace, app.Name))

		if latest.Annotations == nil {
			latest.Annotations = map[string]string{
				utils.ProgressiveSyncSyncedAtStageKey: stageName,
			}
		} else {
			latest.Annotations[utils.ProgressiveSyncSyncedAtStageKey] = stageName
		}

		if err := r.Client.Update(ctx, &latest); err != nil {
			return err
		}

		stageKeyValue := ps.Name + "/" + stageName
		appKeyValue := ps.Name + "/" + app.Name

		r.SyncedAtStage[appKeyValue] = stageName

		val, ok := r.SyncedAppsPerStage[stageKeyValue]
		if !ok {
			r.SyncedAppsPerStage[stageKeyValue] = 1
		} else {
			if val+1 > stageMaxTargets {
				r.SyncedAppsPerStage[stageKeyValue] = stageMaxTargets
			} else {
				r.SyncedAppsPerStage[stageKeyValue] = val + 1
			}
		}

		log.Info("app annotated")
		return nil
	})
	return retryErr
}

// reconcileStage reconcile a ProgressiveSyncStage
func (r *ProgressiveSyncReconciler) reconcileStage(ctx context.Context, ps syncv1alpha1.ProgressiveSync, stage syncv1alpha1.ProgressiveSyncStage) (syncv1alpha1.ProgressiveSync, reconcile.Result, error) {
	log := r.Log.WithValues("progressivesync", fmt.Sprintf("%s/%s", ps.Namespace, ps.Name), "applicationset", ps.Spec.SourceRef.Name, "stage", stage.Name, "syncedAtStage", r.SyncedAtStage)
	requeueDelayOnError := time.Minute * 5

	// Get the clusters to update
	clusters, err := r.getClustersFromSelector(ctx, stage.Targets.Clusters.Selector)
	if err != nil {
		message := "unable to fetch clusters from selector"
		log.Error(err, message)

		// Set stage status
		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &ps)
		// Set ProgressiveSync status
		failed := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, message)
		apimeta.SetStatusCondition(ps.GetStatusConditions(), failed)

		return ps, ctrl.Result{Requeue: true, RequeueAfter: requeueDelayOnError}, err
	}
	log.Info("fetched clusters using label selector", "clusters", utils.GetClustersName(clusters.Items))

	// Get only the Applications owned by the ProgressiveSync targeting the selected clusters
	apps, err := r.getOwnedAppsFromClusters(ctx, clusters, &ps)
	if err != nil {
		message := "unable to fetch apps from clusters"
		log.Error(err, message)

		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &ps)
		failed := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, message)
		apimeta.SetStatusCondition(ps.GetStatusConditions(), failed)
		return ps, ctrl.Result{Requeue: true, RequeueAfter: requeueDelayOnError}, err
	}
	log.Info("fetched apps targeting selected clusters", "apps", utils.GetAppsName(apps))

	// Get the Applications to update
	scheduledApps := scheduler.Scheduler(log, apps, stage, r.SyncedAtStage, ps.Name)

	if len(scheduledApps) == 0 {
		maxTargets, _ := intstr.GetScaledValueFromIntOrPercent(&stage.MaxTargets, len(apps), false)

		healthyApps := utils.GetAppsByHealthStatusCode(apps, health.HealthStatusHealthy)

		if maxTargets > len(healthyApps) {
			maxTargets = len(healthyApps)
		}

		stageKeyValue := ps.Name + "/" + stage.Name
		_, ok := r.SyncedAppsPerStage[stageKeyValue]
		if !ok {
			r.SyncedAppsPerStage[stageKeyValue] = 0
		}

		maxTargets = maxTargets - r.SyncedAppsPerStage[stageKeyValue]

		for i := 0; i < maxTargets; i++ {
			appKeyValue := ps.Name + "/" + healthyApps[i].Name
			r.SyncedAtStage[appKeyValue] = stage.Name
			r.SyncedAppsPerStage[stageKeyValue]++
		}
	}

	for _, s := range scheduledApps {
		log.Info("syncing app", "app", fmt.Sprintf("%s/%s", s.Namespace, s.Name), "sync.status", s.Status.Sync.Status, "health.status", s.Status.Health.Status)

		_, err = r.syncApp(ctx, s.Name)

		if err != nil {
			if !strings.Contains(err.Error(), "another operation is already in progress") {
				message := "failed to sync app"
				log.Error(err, message, "message", err.Error())

				r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &ps)
				// Set ProgressiveSync status
				failed := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, message)
				apimeta.SetStatusCondition(ps.GetStatusConditions(), failed)
				return ps, ctrl.Result{RequeueAfter: requeueDelayOnError}, err
			}
			log.Info("failed to sync app because it is already syncing")
		}
		stageMaxTargets, _ := intstr.GetScaledValueFromIntOrPercent(&stage.MaxTargets, len(apps), false)
		err := r.setSyncedAtAnnotation(ctx, s, stage.Name, stageMaxTargets, ps)
		if err != nil {
			message := "failed at setSyncedAtAnnotation"
			log.Error(err, message, "message", err.Error())

			return ps, ctrl.Result{RequeueAfter: requeueDelayOnError}, err
		}

	}

	if scheduler.IsStageFailed(apps, stage, r.SyncedAtStage, ps.Name) {
		message := fmt.Sprintf("%s stage failed", stage.Name)
		log.Info(message)

		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &ps)

		failed := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, message)
		apimeta.SetStatusCondition(ps.GetStatusConditions(), failed)

		log.Info("sync failed")
		return ps, ctrl.Result{Requeue: true, RequeueAfter: requeueDelayOnError}, nil
	}

	if scheduler.IsStageInProgress(apps, stage, r.SyncedAtStage, ps.Name) {
		message := fmt.Sprintf("%s stage in progress", stage.Name)
		log.Info(message)

		for _, app := range apps {
			log.Info("application details", "app", fmt.Sprintf("%s/%s", app.Namespace, app.Name), "annotations", app.GetAnnotations(), "sync.status", app.Status.Sync.Status, "health.status", app.Status.Health.Status)
		}

		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseProgressing, &ps)

		progress := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
		apimeta.SetStatusCondition(ps.GetStatusConditions(), progress)

		// Stage in progress, we reconcile again until the stage is completed or failed
		return ps, ctrl.Result{Requeue: true}, nil
	}

	if scheduler.IsStageComplete(apps, stage, r.SyncedAtStage, ps.Name) {
		message := fmt.Sprintf("%s stage completed", stage.Name)
		log.Info(message)

		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseSucceeded, &ps)

		progress := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
		apimeta.SetStatusCondition(ps.GetStatusConditions(), progress)

		return ps, ctrl.Result{}, nil

	}

	return ps, ctrl.Result{Requeue: true}, nil
}

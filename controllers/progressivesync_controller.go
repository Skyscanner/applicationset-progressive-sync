/*
Copyright 2021 Skyscanner Limited.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"strings"

	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/json"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/utils"
	applicationset "github.com/argoproj-labs/applicationset/api/v1alpha1"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const RequeueDelayOnError = time.Minute * 5

// ProgressiveSyncReconciler reconciles a ProgressiveSync object
type ProgressiveSyncReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ArgoCDAppClient utils.ArgoCDAppClient
	StateManager    utils.ProgressiveSyncStateManager
	ArgoNamespace   string
}

// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applicationsets,verbs=get;list

// Reconcile performs the reconciling for a single named ProgressiveSync object
func (r *ProgressiveSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log = log.WithValues("progressivesync", req.NamespacedName)
	log.Info("reconciliation loop started")

	var ps syncv1alpha1.ProgressiveSync
	if err := r.Get(ctx, req.NamespacedName, &ps); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If the object is being deleted, cleanup and remove the finalizer
	if !ps.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, ps)
	}

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&ps, syncv1alpha1.ProgressiveSyncFinalizer) {
		controllerutil.AddFinalizer(&ps, syncv1alpha1.ProgressiveSyncFinalizer)
		if err := r.Update(ctx, &ps); err != nil {
			log.Error(err, "failed to update object when adding finalizer")
			return ctrl.Result{}, err
		}
	}

	reconcilePs, result, err := r.reconcile(ctx, ps)

	// Update status after reconciliation.
	if updateStatusErr := r.patchStatus(ctx, reconcilePs); updateStatusErr != nil {
		log.Error(updateStatusErr, "unable to update status after reconciliation")
		return ctrl.Result{Requeue: true}, updateStatusErr
	}

	return result, err

	// pss, _ := r.StateManager.Get(ps.Name)

	// newHash, err := r.calculateHashedSpec(ctx, &ps)
	// if err != nil {
	// 	log.Error(err, "Failed to generate the hash value from the ApplicationSet spec")
	// 	return ctrl.Result{}, err
	// }
	// currentHash := pss.GetHashedSpec()
	// if currentHash != newHash {
	// 	pss.SetHashedSpec(newHash)
	// 	log.Info("Successfully updated the hash value of the ApplicationSet spec", "service", ps.Name)
	// } else {
	// 	log.Info("Hash value is the same as nothing has changed in the ApplicationSet spec", "service", ps.Name)
	// }

	// latest := ps
	// var result reconcile.Result
	// var reconcileErr error

	// for _, stage := range ps.Spec.Stages {
	// 	log = log.WithValues("stage", stage.Name)

	// 	latest, result, reconcileErr = r.reconcileStage(ctx, latest, stage, pss)
	// 	if err := r.updateStatusWithRetry(ctx, &latest); err != nil {
	// 		return ctrl.Result{}, err
	// 	}

	// 	if result.Requeue || reconcileErr != nil {
	// 		log.Info("requeuing stage")
	// 		return result, reconcileErr
	// 	}

	// }

	// // Progressive sync completed
	// completed := latest.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesCompleteReason, "All stages completed")
	// apimeta.SetStatusCondition(latest.GetStatusConditions(), completed)
	// if err := r.updateStatusWithRetry(ctx, &latest); err != nil {
	// 	log.Error(err, "failed to update object status")
	// 	return ctrl.Result{}, err
	// }
	// log.Info("sync completed")
	// return ctrl.Result{}, nil
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
	log := log.FromContext(ctx)

	app, ok := o.(*argov1alpha1.Application)
	if !ok {
		err := fmt.Errorf("expected application, got %T", o)
		log.Error(err, "failed to convert object to application")
		return nil
	}

	if err := r.List(ctx, &list); err != nil {
		log.Error(err, "failed to list ProgressiveSync")
		return nil
	}

	for _, pr := range list.Items {
		if pr.Owns(app.GetOwnerReferences()) {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pr.Namespace,
				Name:      pr.Name,
			}})
			log.Info("application changed", "app", fmt.Sprintf("%s/%s", app.Namespace, app.Name), "sync.status", app.Status.Sync.Status, "health.status", app.Status.Health.Status)
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
	log := log.FromContext(ctx)

	s, ok := o.(*corev1.Secret)
	if !ok {
		err := fmt.Errorf("expected secret, got %T", o)
		log.Error(err, "failed to convert object to secret")
		return nil
	}

	log.Info("received secret event", "name", s.Name, "Namespace", s.Namespace)

	if !utils.IsArgoCDCluster(s.GetLabels()) {
		return nil
	}

	if err := r.List(ctx, &prList); err != nil {
		log.Error(err, "failed to list ProgressiveSync")
		return nil
	}
	if err := r.List(ctx, &appList); err != nil {
		log.Error(err, "failed to list Application")
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
					To avoid that, we use a map to store for which ProgressiveSync object we already triggered the reconciliation loop.
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

	argoSelector := metav1.AddLabelToSelector(&selector, consts.ArgoCDSecretTypeLabel, consts.ArgoCDSecretTypeCluster)
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

// calculateHashValue returns a hash value string for the ApplicationSet spec
func (r *ProgressiveSyncReconciler) calculateHashedSpec(ctx context.Context, ps *syncv1alpha1.ProgressiveSync) (string, error) {
	key := client.ObjectKeyFromObject(ps)
	latest := syncv1alpha1.ProgressiveSync{}
	if err := r.Client.Get(ctx, key, &latest); err != nil {
		r.Log.Error(err, "Failed to retrieve the ProgressiveSync object while calculating the hash value")
		return "", err
	}

	appSet := applicationset.ApplicationSet{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: r.ArgoNamespace, Name: latest.Spec.SourceRef.Name}, &appSet); err != nil {
		r.Log.Error(err, "Failed to retrieve the ApplicationSet object")
		return "", err
	}

	appSetSpecInBytes, err := json.Marshal(appSet.Spec)
	if err != nil {
		r.Log.Error(err, "Failed to encode the ApplicanSet spec")
		return "", err
	}

	hashedSpecInBytes := md5.Sum(appSetSpecInBytes)
	hashedSpec := hex.EncodeToString(hashedSpecInBytes[:])

	r.Log.Info("Successfully calculate the hash value of the Spec")
	return hashedSpec, nil
}

// getOwnedAppsFromClusters returns a list of Applications targeting the specified clusters and owned by the specified ProgressiveSync
func (r *ProgressiveSyncReconciler) getOwnedAppsFromClusters(ctx context.Context, clusters corev1.SecretList, ps syncv1alpha1.ProgressiveSync) ([]argov1alpha1.Application, error) {
	log := log.FromContext(ctx)

	var apps []argov1alpha1.Application
	var appList argov1alpha1.ApplicationList

	if err := r.List(ctx, &appList); err != nil {
		log.Error(err, "failed to list Application")
		return apps, err
	}

	for _, cluster := range clusters.Items {
		for _, app := range appList.Items {
			if ps.Owns(app.GetOwnerReferences()) && string(cluster.Data["server"]) == app.Spec.Destination.Server {
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
func (r *ProgressiveSyncReconciler) syncApp(ctx context.Context, app argov1alpha1.Application) error {
	syncReq := applicationpkg.ApplicationSyncRequest{
		Name: &app.Name,
	}

	_, err := r.ArgoCDAppClient.Sync(ctx, &syncReq)
	if err != nil && !strings.Contains(err.Error(), "another operation is already in progress") {
		return err
	}

	return nil
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

// markAppAsSynced marks the app as synced in the specified stage
func (r *ProgressiveSyncReconciler) markAppAsSynced(ctx context.Context, app argov1alpha1.Application, stage syncv1alpha1.Stage, pss utils.ProgressiveSyncState) error {

	log := r.Log.WithValues("stage", stage.Name, "app", fmt.Sprintf("%s/%s", app.Namespace, app.Name))

	pss.MarkAppAsSynced(app, stage)

	log.Info("app marked as synced")

	return nil
}

// reconcile performs the actual reconciliation logic
func (r *ProgressiveSyncReconciler) reconcile(ctx context.Context, ps syncv1alpha1.ProgressiveSync) (syncv1alpha1.ProgressiveSync, ctrl.Result, error) {

	log := log.FromContext(ctx)

	// Initialize the ConfigMap holding the ProgressiveSync status
	cmName := fmt.Sprintf("%s-ps-state", ps.Name)
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: ps.Namespace,
		},
		Data: make(map[string]string, 0),
	}

	// Get the current state ConfigMap
	err := r.Get(ctx, client.ObjectKeyFromObject(&cm), &cm)
	if client.IgnoreNotFound(err) != nil {
		log.Error(err, "unable to retrieve the state configmap", "configmap", cmName)
		return ps, ctrl.Result{Requeue: true}, err
	}

	// Create a state ConfigMap if it doesn't exist
	if client.IgnoreNotFound(err) == nil {
		if createErr := r.Client.Create(ctx, &cm, &client.CreateOptions{}); createErr != nil {
			log.Error(createErr, "unable to create the state configmap", "configmap", cmName)
			return ps, ctrl.Result{Requeue: true}, createErr
		}
	}

	if ps.Status.ObservedGeneration != ps.Generation {
		ps.Status.ObservedGeneration = ps.Generation
		ps = syncv1alpha1.ProgressiveSyncProgressing(ps)
		if updateStatusErr := r.patchStatus(ctx, ps); updateStatusErr != nil {
			log.Error(updateStatusErr, "unable to update status after generation update")
			return ps, ctrl.Result{Requeue: true}, updateStatusErr
		}
	}

	// TODO #63:  Calculate hash to check if it's a new progressive sync or an old one

	for _, stage := range ps.Spec.Stages {

		ps.Status.LastSyncedStage = stage.Name

		stageStatus, err := r.reconcileStage(ctx, ps, stage)

		// An error indicates the stage failed the reconciliation
		if err != nil {
			log.Error(err, "unable to reconcile stage", "stage", stage.Name)
			ps.Status.LastSyncedStageStatus = syncv1alpha1.StageStatus(syncv1alpha1.StageStatusFailed)
			return syncv1alpha1.ProgressiveSyncNotReady(ps, syncv1alpha1.StageFailedReason, err.Error()), ctrl.Result{Requeue: true}, err
		}

		switch {
		case stageStatus == syncv1alpha1.StageStatus(syncv1alpha1.StageStatusCompleted):
			{
				ps.Status.LastSyncedStageStatus = syncv1alpha1.StageStatus(syncv1alpha1.StageStatusCompleted)
			}
		case stageStatus == syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing):
			{
				ps.Status.LastSyncedStageStatus = syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing)
				return syncv1alpha1.ProgressiveSyncProgressing(ps), ctrl.Result{Requeue: true}, nil
			}
		}
	}

	return syncv1alpha1.ProgressiveSyncReady(ps), ctrl.Result{}, nil
}

// reconcileStage observes the state of the world and sync the desired number of apps
func (r *ProgressiveSyncReconciler) reconcileStage(ctx context.Context, ps syncv1alpha1.ProgressiveSync, stage syncv1alpha1.Stage) (syncv1alpha1.StageStatus, error) {
	// A cluster is represented in ArgoCD by a secret
	// Get the ArgoCD secrets selected by the label selector
	selectedCluster, err := r.getClustersFromSelector(ctx, stage.Targets.Clusters.Selector)
	if err != nil {
		return syncv1alpha1.StageStatus(syncv1alpha1.StageStatusFailed), err
	}

	// Get the ArgoCD apps targeting the selected clusters
	selectedApps, err := r.getOwnedAppsFromClusters(ctx, selectedCluster, ps)
	if err != nil {
		return syncv1alpha1.StageStatus(syncv1alpha1.StageStatusFailed), err
	}

	outOfSyncApps := utils.GetAppsBySyncStatusCode(selectedApps, argov1alpha1.SyncStatusCodeOutOfSync)

	// If there are no OutOfSync apps then the Stage in completed
	if len(outOfSyncApps) == 0 {
		return syncv1alpha1.StageStatus(syncv1alpha1.StageStatusCompleted), nil
	}

	// TODO: calculate maxTargets and maxParallel as percentage

	progressingApps := utils.GetAppsByHealthStatusCode(selectedApps, health.HealthStatusProgressing)

	maxTargets := int(stage.MaxTargets)
	maxParallel := int(stage.MaxParallel)

	// If we reached the maximum number of progressing apps for the stage
	// then the Stage is progressing
	if len(progressingApps) == maxTargets {
		return syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing), nil
	}

	// If there is an external process triggering a sync,
	// maxParallel - len(progressingApps) might actually be greater than len(outOfSyncApps)
	// causing the runtime to panic
	maxSync := maxParallel - len(progressingApps)
	if maxSync > len(outOfSyncApps) {
		maxSync = len(outOfSyncApps)
	}

	//TODO: read the configmap to calculate syncedInCurrentStage
	var syncedInCurrentStage []argov1alpha1.Application

	// Consider the following scenario
	//
	// maxTargets = 3
	// maxParallel = 3
	// outOfSyncApps = 4
	// syncedInCurrentStage = 2
	// progressingApps = 1
	//
	// This scenario makes maxSync = 2
	//
	// Without the following logic we would end up
	// with a total of 4 applications synced in the stage
	if maxSync+len(syncedInCurrentStage) > maxTargets {
		maxSync = maxTargets - len(syncedInCurrentStage)
	}

	// Sync the desired number of apps
	for i := 0; i < maxSync; i++ {
		err := r.syncApp(ctx, outOfSyncApps[i])
		if err != nil {
			return syncv1alpha1.StageStatus(syncv1alpha1.StageStatusFailed), err
		}
	}

	return syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing), nil
}

// func (r *ProgressiveSyncReconciler) reconcileStage(ctx context.Context, ps syncv1alpha1.ProgressiveSync, stage syncv1alpha1.ProgressiveSyncStage, pss utils.ProgressiveSyncState) (syncv1alpha1.ProgressiveSync, reconcile.Result, error) {
// 	log := r.Log.WithValues("progressivesync", fmt.Sprintf("%s/%s", ps.Namespace, ps.Name), "applicationset", ps.Spec.SourceRef.Name, "stage", stage.Name, "syncedAtStage", pss)

// 	// Get the clusters to update
// 	clusters, err := r.getClustersFromSelector(ctx, stage.Targets.Clusters.Selector)
// 	if err != nil {
// 		message := "unable to fetch clusters from selector"
// 		log.Error(err, message)

// 		// Set stage status
// 		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &ps)
// 		// Set ProgressiveSync status
// 		failed := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, message)
// 		apimeta.SetStatusCondition(ps.GetStatusConditions(), failed)

// 		return ps, ctrl.Result{Requeue: true, RequeueAfter: RequeueDelayOnError}, err
// 	}
// 	log.Info("fetched clusters using label selector", "clusters", utils.GetClustersName(clusters.Items))

// 	// Get only the Applications owned by the ProgressiveSync targeting the selected clusters
// 	apps, err := r.getOwnedAppsFromClusters(ctx, clusters, &ps)
// 	if err != nil {
// 		message := "unable to fetch apps from clusters"
// 		log.Error(err, message)

// 		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &ps)
// 		failed := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, message)
// 		apimeta.SetStatusCondition(ps.GetStatusConditions(), failed)
// 		return ps, ctrl.Result{Requeue: true, RequeueAfter: RequeueDelayOnError}, err
// 	}
// 	log.Info("fetched apps targeting selected clusters", "apps", utils.GetAppsName(apps))

// 	pss.RefreshState(apps, stage)

// 	// Get the Applications to update
// 	scheduledApps := scheduler.Scheduler(log, apps, stage, pss)

// 	for _, s := range scheduledApps {
// 		log.Info("syncing app", "app", fmt.Sprintf("%s/%s", s.Namespace, s.Name), "sync.status", s.Status.Sync.Status, "health.status", s.Status.Health.Status)

// 		_, err = r.syncApp(ctx, s.Name)

// 		if err != nil {
// 			if !strings.Contains(err.Error(), "another operation is already in progress") {
// 				message := "failed to sync app"
// 				log.Error(err, message, "message", err.Error())

// 				r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &ps)
// 				// Set ProgressiveSync status
// 				failed := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, message)
// 				apimeta.SetStatusCondition(ps.GetStatusConditions(), failed)
// 				return ps, ctrl.Result{RequeueAfter: RequeueDelayOnError}, err
// 			}
// 			log.Info("failed to sync app because it is already syncing")
// 		}
// 		err := r.markAppAsSynced(ctx, s, stage, pss)
// 		if err != nil {
// 			message := "failed at markAppAsSynced"
// 			log.Error(err, message, "message", err.Error())

// 			return ps, ctrl.Result{RequeueAfter: RequeueDelayOnError}, err
// 		}

// 	}

// 	if scheduler.IsStageFailed(apps, stage, pss) {
// 		message := fmt.Sprintf("%s stage failed", stage.Name)
// 		log.Info(message)

// 		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseFailed, &ps)

// 		failed := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, message)
// 		apimeta.SetStatusCondition(ps.GetStatusConditions(), failed)

// 		log.Info("sync failed")
// 		return ps, ctrl.Result{Requeue: true, RequeueAfter: RequeueDelayOnError}, nil
// 	}

// 	if scheduler.IsStageInProgress(apps, stage, pss) {
// 		message := fmt.Sprintf("%s stage in progress", stage.Name)
// 		log.Info(message)

// 		for _, app := range apps {
// 			log.Info("application details", "app", fmt.Sprintf("%s/%s", app.Namespace, app.Name), "sync.status", app.Status.Sync.Status, "health.status", app.Status.Health.Status)
// 		}

// 		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseProgressing, &ps)

// 		progress := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
// 		apimeta.SetStatusCondition(ps.GetStatusConditions(), progress)

// 		// Stage in progress, we reconcile again until the stage is completed or failed
// 		return ps, ctrl.Result{Requeue: true}, nil
// 	}

// 	if scheduler.IsStageComplete(apps, stage, pss) {
// 		message := fmt.Sprintf("%s stage completed", stage.Name)
// 		log.Info(message)

// 		r.updateStageStatus(ctx, stage.Name, message, syncv1alpha1.PhaseSucceeded, &ps)

// 		progress := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
// 		apimeta.SetStatusCondition(ps.GetStatusConditions(), progress)

// 		return ps, ctrl.Result{}, nil

// 	}

// 	message := fmt.Sprintf("%s is in an unknown state!", stage.Name)
// 	log.Info(message)
// 	return ps, ctrl.Result{Requeue: true}, nil
// }

//reconcileDelete deletes the ConfigMap holding the ProgressiveSync object state
// before removing the finalizer
func (r *ProgressiveSyncReconciler) reconcileDelete(ctx context.Context, ps syncv1alpha1.ProgressiveSync) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	//TODO: remove configmap

	controllerutil.RemoveFinalizer(&ps, syncv1alpha1.ProgressiveSyncFinalizer)
	if err := r.Update(ctx, &ps); err != nil {
		log.Error(err, "failed to update object when removing finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

//patchStatus updates the ProgressiveSync object using a MergeFrom strategy
func (r *ProgressiveSyncReconciler) patchStatus(ctx context.Context, ps syncv1alpha1.ProgressiveSync) error {
	key := client.ObjectKeyFromObject(&ps)
	latest := syncv1alpha1.ProgressiveSync{}

	if err := r.Client.Get(ctx, key, &latest); err != nil {
		return err
	}

	return r.Client.Status().Patch(ctx, &ps, client.MergeFrom(&latest))
}

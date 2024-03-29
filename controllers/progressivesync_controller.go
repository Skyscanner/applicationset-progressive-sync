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
	"strings"

	"fmt"
	"time"

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
}

// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=argoproj.skyscanner.net,resources=progressivesyncs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="argoproj.io",resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applicationsets,verbs=get;list

// Reconcile performs the reconciling for a single named ProgressiveSync object
func (r *ProgressiveSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("progressive sync started")

	var ps syncv1alpha1.ProgressiveSync
	if err := r.Get(ctx, req.NamespacedName, &ps); err != nil {
		log.Error(err, "unable to get the progressive sync object")
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
			log.Error(err, "unable to update object when adding finalizer")
			return ctrl.Result{}, err
		}
	}

	reconciledPs, result, err := r.reconcile(ctx, ps)

	// Update status after reconciliation.
	if updateStatusErr := r.patchStatus(ctx, reconciledPs); updateStatusErr != nil {
		log.Error(updateStatusErr, "unable to update status after reconciliation")
		return ctrl.Result{Requeue: true}, updateStatusErr
	}

	return result, err
}

// SetupWithManager adds the reconciler to the manager, so that it gets started when the manager is started.
func (r *ProgressiveSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {

	mapOwnerKey := ".metadata.controller"
	apiGVStr := syncv1alpha1.GroupVersion.String()

	// Add an index to allow our reconciler to quickly look up configmaps by their owner.
	// See https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html#setup
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.ConfigMap{}, mapOwnerKey, func(rawObj client.Object) []string {
		// Get the object and extract the owner
		cm := rawObj.(*corev1.ConfigMap)
		owner := metav1.GetControllerOf(cm)
		if owner == nil {
			return nil
		}
		// Make sure it's a ProgressiveSync
		if owner.APIVersion != apiGVStr || owner.Kind != "ProgressiveSync" {
			return nil
		}

		// If so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&syncv1alpha1.ProgressiveSync{}).
		Owns(&corev1.ConfigMap{}).
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

	// We trigger a reconciliation loop on an Application event if:
	// the Application owner is referenced by a ProgressiveSync object

	var requests []reconcile.Request
	var psList syncv1alpha1.ProgressiveSyncList
	ctx := context.Background()
	log := log.FromContext(ctx)

	app, ok := o.(*argov1alpha1.Application)
	if !ok {
		err := fmt.Errorf("expected application, got %T", o)
		log.Error(err, "unable to convert object into application")
		return nil
	}

	if err := r.List(ctx, &psList); err != nil {
		log.Error(err, "unable to list argov1alpha1.ApplicationList")
		return nil
	}

	for _, ps := range psList.Items {
		if ps.Owns(app.GetOwnerReferences()) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: ps.Namespace,
					Name:      ps.Name,
				},
			})
		}
	}

	return requests
}

// requestsForSecretChange returns a reconcile request when a Secret changes
func (r *ProgressiveSyncReconciler) requestsForSecretChange(o client.Object) []reconcile.Request {

	// We trigger a reconciliation loop on a Secret event if:
	// - the Secret is an ArgoCD cluster, AND
	// - there is an Application targeting that secret/cluster, AND
	// - that Application owner is referenced by a ProgressiveSync object

	var requests []reconcile.Request
	var psList syncv1alpha1.ProgressiveSyncList
	var appList argov1alpha1.ApplicationList
	requestsMap := make(map[types.NamespacedName]bool)
	ctx := context.Background()
	log := log.FromContext(ctx)

	s, ok := o.(*corev1.Secret)
	if !ok {
		err := fmt.Errorf("expected secret, got %T", o)
		log.Error(err, "unable to convert object into secret")
		return nil
	}

	if !utils.IsArgoCDCluster(s.GetLabels()) {
		return nil
	}

	if err := r.List(ctx, &psList); err != nil {
		log.Error(err, "unable to list syncv1alpha1.ProgressiveSyncList")
		return nil
	}
	if err := r.List(ctx, &appList); err != nil {
		log.Error(err, "unable to list argov1alpha1.ApplicationList")
		return nil
	}

	for _, ps := range psList.Items {
		for _, app := range appList.Items {
			if app.Spec.Destination.Server == string(s.Data["server"]) && ps.Owns(app.GetOwnerReferences()) {

				// Consider the following scenario:
				// - two Applications
				// - owned by the same ApplicationSet
				// - referenced by the same ProgressiveSync
				// - targeting the same cluster
				//
				// In this scenario, we would trigger the reconciliation loop twice.
				// To avoid that, we use a map to store
				// for which ProgressiveSync object we already triggered the reconciliation loop.
				namespacedName := types.NamespacedName{Name: ps.Name, Namespace: ps.Namespace}
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
	log := log.FromContext(ctx)

	argoSelector := metav1.AddLabelToSelector(&selector, consts.ArgoCDSecretTypeLabel, consts.ArgoCDSecretTypeCluster)
	labels, err := metav1.LabelSelectorAsSelector(argoSelector)
	if err != nil {
		log.Error(err, "unable to convert clusters selector into labels")
		return corev1.SecretList{}, err
	}

	if err = r.List(ctx, &secrets, client.MatchingLabelsSelector{Selector: labels}); err != nil {
		log.Error(err, "unable to select clusters using labels selector")
		return corev1.SecretList{}, err
	}

	// https://github.com/Skyscanner/applicationset-progressive-sync/issues/9 will provide a better sorting
	utils.SortSecretsByName(&secrets)

	return secrets, nil
}

// getOwnedAppsFromClusters returns a list of Applications targeting the specified clusters and owned by the specified ProgressiveSync
func (r *ProgressiveSyncReconciler) getOwnedAppsFromClusters(ctx context.Context, clusters corev1.SecretList, ps syncv1alpha1.ProgressiveSync) ([]argov1alpha1.Application, error) {
	log := log.FromContext(ctx)

	var apps []argov1alpha1.Application
	var appList argov1alpha1.ApplicationList

	if err := r.List(ctx, &appList); err != nil {
		log.Error(err, "unable to list argov1alpha1.ApplicationList")
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

// syncApp sends a sync request for the target app
func (r *ProgressiveSyncReconciler) syncApp(ctx context.Context, app argov1alpha1.Application, syncOpts syncv1alpha1.SyncOptions) error {
	syncReq := applicationpkg.ApplicationSyncRequest{
		Name:  &app.Name,
		Prune: syncOpts.Prune,
	}
	_, err := r.ArgoCDAppClient.Sync(ctx, &syncReq)
	if err != nil && !strings.Contains(err.Error(), "another operation is already in progress") {
		return err
	}

	return nil
}

// reconcile performs the actual reconciliation logic
func (r *ProgressiveSyncReconciler) reconcile(ctx context.Context, ps syncv1alpha1.ProgressiveSync) (syncv1alpha1.ProgressiveSync, ctrl.Result, error) {

	log := log.FromContext(ctx)

	// Observe ProgressiveSync generation
	if ps.Status.ObservedGeneration != ps.Generation {
		ps.Status.ObservedGeneration = ps.Generation
		return syncv1alpha1.ProgressiveSyncProgressing(ps), ctrl.Result{Requeue: true}, nil
	}

	// Read the latest state
	state, err := r.ReadStateMap(ctx, ps)
	if err != nil {
		log.Error(err, "unable to read state configmap")
		return ps, ctrl.Result{RequeueAfter: RequeueDelayOnError}, err
	}

	// Get the reference ApplicationSet
	var appSet applicationset.ApplicationSet
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name:      ps.Spec.AppSetRef.Name,
		Namespace: ps.Namespace,
	}, &appSet); err != nil {
		log.Error(err, "unable to get the referenced application set")
		return ps, ctrl.Result{RequeueAfter: RequeueDelayOnError}, err
	}

	// Observe ApplicationSet hash
	currentAppSetHash := utils.ComputeHash(appSet.Spec)
	if currentAppSetHash != state.AppSetHash {
		state.AppSetHash = currentAppSetHash
		if updateMapErr := r.UpdateStateMap(ctx, ps, state); err != nil {
			log.Error(updateMapErr, "unable to update state map after appset hash update")
			return ps, ctrl.Result{RequeueAfter: RequeueDelayOnError}, updateMapErr
		}

		return syncv1alpha1.ProgressiveSyncProgressing(ps), ctrl.Result{Requeue: true}, nil
	}

	for _, stage := range ps.Spec.Stages {
		// An error indicates the stage failed to reconcile
		stageStatus, err := r.reconcileStage(ctx, ps, stage)
		if err != nil {
			log.Error(err, "unable to reconcile stage", "stage", stage.Name)
			ps.Status.LastSyncedStageStatus = syncv1alpha1.StageStatusFailed
			return syncv1alpha1.ProgressiveSyncNotReady(ps, syncv1alpha1.StageFailedReason, err.Error()), ctrl.Result{RequeueAfter: RequeueDelayOnError}, err
		}

		ps.Status.LastSyncedStage = stage.Name
		ps.Status.LastSyncedStageStatus = stageStatus
		log.Info("stage reconciled", "stage", stage.Name, "status", stageStatus)

		if stageStatus == syncv1alpha1.StageStatusProgressing {
			return syncv1alpha1.ProgressiveSyncProgressing(ps), ctrl.Result{Requeue: true}, nil
		}
	}

	log.Info("progressive sync completed")
	return syncv1alpha1.ProgressiveSyncReady(ps), ctrl.Result{}, nil
}

// reconcileStage observes the state of the world and sync the desired number of apps
func (r *ProgressiveSyncReconciler) reconcileStage(ctx context.Context, ps syncv1alpha1.ProgressiveSync, stage syncv1alpha1.Stage) (syncv1alpha1.StageStatus, error) {
	log := log.FromContext(ctx)

	// A cluster is represented in ArgoCD by a secret
	// Get the ArgoCD secrets selected by the label selector
	selectedClusters, err := r.getClustersFromSelector(ctx, stage.Targets.Clusters.Selector)
	if err != nil {
		return syncv1alpha1.StageStatusFailed, err
	}

	// Skip stage if no clusters targeted
	if len(selectedClusters.Items) == 0 {
		return syncv1alpha1.StageStatusCompleted, nil
	}

	// Get the ArgoCD apps targeting the selected clusters
	selectedApps, err := r.getOwnedAppsFromClusters(ctx, selectedClusters, ps)
	if err != nil {
		return syncv1alpha1.StageStatusFailed, err
	}

	// There might be a race condition where:
	// 1. The repoURL points to a new revision
	// 2. ArgoCD starts marking the Applications as OutOfSync
	// 3. As soon as the first Application is updated,
	//	  the progressive sync controller starts the reconciliation loop
	// 4. The controller picks up the first stage and makes decision based on the state of the world
	//
	// In this case, the controller might take a decision
	// while ArgoCD is still updating the Applications Status.
	// To prevent this scenario, we make sure all the selectedApps
	// have the same revision before progressing with the stage.
	if !utils.HaveSameRevision(selectedApps) {
		return syncv1alpha1.StageStatusProgressing, nil
	}

	// Load the state map
	state, err := r.ReadStateMap(ctx, ps)
	if err != nil {
		log.Error(err, "unabled to load the state map")
		return syncv1alpha1.StageStatusFailed, err
	}

	maxTargets := int(stage.MaxTargets)
	maxParallel := int(stage.MaxParallel)

	// Consider the scenario where we have 5 apps - 4 OutOfSync and 1 Synced - and a stage with MaxTargets = 3.
	// Without keeping track at which stage the app synced, we can't compute how many applications we have to update in the current stage
	// because it would not be possible to know if the app synced at this stage or in the previous one.
	var syncedInCurrentStage []argov1alpha1.Application

	// During normal operations, the controller assumes
	// that is the only entity in charge of syncing apps.
	//
	// This mean the controller expect to get out-of-sync apps,
	// triggering a sync and watch for the apps status.
	//
	// There might be situation where this is not true, for example if:
	// - we're missing a kubernetes events,
	//   so for example we missed a out-of-sync -> synced transition
	// - there is an external entity, for example a user, triggering the sync via ArgoCD
	//
	// To recover against those scenarios, we need to adopt any synced apps
	// missing from the state configmap and assign the current stage to it.
	syncedApps := utils.GetAppsBySyncStatusCode(selectedApps, argov1alpha1.SyncStatusCodeSynced)

	for _, app := range syncedApps {
		// Check if there is an entry in the state map for the synced app
		appState, ok := state.Apps[app.Name]

		// If we don't have a state for a synced app
		// it means it was synced by an external process.
		// Adopt the app by setting its synced stage to the current one.
		if !ok {
			state.Apps[app.Name] = AppState{
				SyncedAtStage: stage.Name,
			}
			syncedInCurrentStage = append(syncedInCurrentStage, app)
			continue
		}

		// If the synced app has an entry in the state map,
		// check if it was synced at the current stage
		if appState.SyncedAtStage == stage.Name {
			syncedInCurrentStage = append(syncedInCurrentStage, app)
		}
	}

	// Update the state map
	if err := r.UpdateStateMap(ctx, ps, state); err != nil {
		log.Error(err, "unabled to update the state map")
		return syncv1alpha1.StageStatusFailed, err
	}

	// If any adopted app is failed, fail the stage
	if len(utils.GetAppsByHealthStatusCode(syncedInCurrentStage, health.HealthStatusDegraded)) > 0 {
		return syncv1alpha1.StageStatusFailed,
			fmt.Errorf("apps %s health status degraded",
				utils.GetAppsName(utils.GetAppsByHealthStatusCode(syncedInCurrentStage, health.HealthStatusDegraded)))
	}

	// To be on the safe side, the controller looks at ALL the progressing Applications selected by the stage selector.
	// This to avoid having too many Applications in progress triggered by an external process.
	progressingApps := utils.GetAppsByHealthStatusCode(syncedApps, health.HealthStatusProgressing)

	// If we reached the maximum number of progressing apps for the stage
	// then the stage is progressing
	if len(progressingApps) >= maxTargets {
		return syncv1alpha1.StageStatusProgressing, nil
	}

	outOfSyncApps := utils.GetAppsBySyncStatusCode(selectedApps, argov1alpha1.SyncStatusCodeOutOfSync)

	// If there are no out-of-sync apps then the stage is completed
	if len(outOfSyncApps) == 0 {
		return syncv1alpha1.StageStatusCompleted, nil
	}

	// If the number of apps synced in the current stage
	// is equal or greater than the maximum number of targets to sync,
	// there is nothing else to do.
	if len(syncedInCurrentStage) >= maxTargets {
		return syncv1alpha1.StageStatusCompleted, nil
	}

	// parallel is the number of Applications to sync at this stage
	parallel := maxParallel - len(progressingApps)

	// If there is an external process triggering a sync,
	// maxParallel - len(progressingApps) might actually be greater than len(outOfSyncApps)
	// causing the runtime to panic
	parallel = utils.Min(parallel, len(outOfSyncApps))

	// Consider the following scenario
	//
	// maxTargets = 3
	// maxParallel = 3
	// outOfSyncApps = 4
	// syncedInCurrentStage = 2
	// progressingApps = 1
	//
	// Without the following logic we would end up
	// with a total of 4 applications synced in the stage
	parallel = utils.Min(parallel, maxTargets-len(syncedInCurrentStage))

	// Sync the desired number of apps
	for i := 0; i < parallel; i++ {
		if err := r.syncApp(ctx, outOfSyncApps[i], ps.Spec.SyncOptions); err != nil {
			return syncv1alpha1.StageStatusFailed, err
		}
		state.Apps[outOfSyncApps[i].Name] = AppState{
			SyncedAtStage: stage.Name,
		}
	}

	// Update the state map
	if err := r.UpdateStateMap(ctx, ps, state); err != nil {
		log.Error(err, "unabled to update the state map")
		return syncv1alpha1.StageStatusFailed, err
	}

	return syncv1alpha1.StageStatusProgressing, nil
}

//reconcileDelete deletes the configmap holding the ProgressiveSync object state
// before removing the finalizer
func (r *ProgressiveSyncReconciler) reconcileDelete(ctx context.Context, ps syncv1alpha1.ProgressiveSync) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if err := r.DeleteStateMap(ctx, ps); err != nil {
		log.Error(err, "unable to delete the state configmap")
		return ctrl.Result{Requeue: true}, err
	}

	controllerutil.RemoveFinalizer(&ps, syncv1alpha1.ProgressiveSyncFinalizer)
	if err := r.Update(ctx, &ps); err != nil {
		log.Error(err, "unable to update object when removing finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

//patchStatus updates the ProgressiveSync object using a MergeFrom strategy
func (r *ProgressiveSyncReconciler) patchStatus(ctx context.Context, ps syncv1alpha1.ProgressiveSync) error {
	var latest syncv1alpha1.ProgressiveSync

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(&ps), &latest); err != nil {
		return err
	}

	patch := client.MergeFrom(latest.DeepCopy())
	latest.Status = ps.Status

	return r.Client.Status().Patch(ctx, &latest, patch)
}

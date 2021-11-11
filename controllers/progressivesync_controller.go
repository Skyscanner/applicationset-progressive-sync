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
	"github.com/Skyscanner/applicationset-progressive-sync/internal/state"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/utils"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		log.Error(err, "unable to convert object into application")
		return nil
	}

	if err := r.List(ctx, &list); err != nil {
		log.Error(err, "unable to list argov1alpha1.ApplicationList")
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

	for _, pr := range psList.Items {
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

// reconcile performs the actual reconciliation logic
func (r *ProgressiveSyncReconciler) reconcile(ctx context.Context, ps syncv1alpha1.ProgressiveSync) (syncv1alpha1.ProgressiveSync, ctrl.Result, error) {

	log := log.FromContext(ctx)

	cmKey := types.NamespacedName{
		Name:      fmt.Sprintf("progressive-sync-%s-state", ps.Name),
		Namespace: ps.Namespace,
	}
	if err := r.createStateMap(ctx, ps, cmKey); err != nil {
		return ps, ctrl.Result{RequeueAfter: RequeueDelayOnError}, err
	}

	if ps.Status.ObservedGeneration != ps.Generation {
		ps.Status.ObservedGeneration = ps.Generation
		ps = syncv1alpha1.ProgressiveSyncProgressing(ps)
		if updateStatusErr := r.patchStatus(ctx, ps); updateStatusErr != nil {
			log.Error(updateStatusErr, "unable to update status after generation update")
			return ps, ctrl.Result{Requeue: true}, updateStatusErr
		}
	}

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

	// If there are no out-of-sync apps then the stage is completed
	if len(outOfSyncApps) == 0 {
		return syncv1alpha1.StageStatus(syncv1alpha1.StageStatusCompleted), nil
	}

	progressingApps := utils.GetAppsByHealthStatusCode(selectedApps, health.HealthStatusProgressing)
	maxTargets := int(stage.MaxTargets)
	maxParallel := int(stage.MaxParallel)

	// If we reached the maximum number of progressing apps for the stage
	// then the stage is progressing
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

//reconcileDelete deletes the configmap holding the ProgressiveSync object state
// before removing the finalizer
func (r *ProgressiveSyncReconciler) reconcileDelete(ctx context.Context, ps syncv1alpha1.ProgressiveSync) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	key := types.NamespacedName{
		Name:      fmt.Sprintf("progressive-sync-%s-state", ps.Name),
		Namespace: ps.Namespace,
	}

	if err := r.deleteStateMap(ctx, key); err != nil {
		log.Error(err, "unable to delete the state configmap", "configmap", key.Name)
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
	key := client.ObjectKeyFromObject(&ps)
	latest := syncv1alpha1.ProgressiveSync{}

	if err := r.Client.Get(ctx, key, &latest); err != nil {
		return err
	}

	return r.Client.Status().Patch(ctx, &ps, client.MergeFrom(&latest))
}

// CreateStateMap creates the state configmap
func (r *ProgressiveSyncReconciler) createStateMap(ctx context.Context, ps syncv1alpha1.ProgressiveSync, key client.ObjectKey) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}

	// Set the ownership
	if err := ctrl.SetControllerReference(&ps, &cm, r.Scheme); err != nil {
		return err
	}

	if err := r.Create(ctx, &cm); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

// DeleteStateMap deletes the state configmap
func (r *ProgressiveSyncReconciler) deleteStateMap(ctx context.Context, key client.ObjectKey) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	if err := r.Delete(ctx, &cm, &client.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// ReadStateMap reads the state configmap and returns the state data structure
func (r *ProgressiveSyncReconciler) readStateMap(ctx context.Context, key client.ObjectKey) (state.StateData, error) {
	var stateData state.StateData
	var cm corev1.ConfigMap

	if err := r.Get(ctx, key, &cm); err != nil {
		return stateData, err
	}

	if err := yaml.Unmarshal([]byte(cm.Data["appSetHash"]), &stateData.AppSetHash); err != nil {
		return stateData, err
	}

	if err := yaml.Unmarshal([]byte(cm.Data["clusters"]), &stateData.Clusters); err != nil {
		return stateData, err
	}

	return stateData, nil
}

// UpdateStateMap writes the state data structure into the state configmap
func (r *ProgressiveSyncReconciler) updateStateMap(ctx context.Context, key client.ObjectKey, state state.StateData) error {

	appSetHash, err := yaml.Marshal(state.AppSetHash)
	if err != nil {
		return err
	}

	clusters, err := yaml.Marshal(state.Clusters)
	if err != nil {
		return err
	}

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: map[string]string{
			"appSetHash": string(appSetHash),
			"clusters":   string(clusters),
		},
	}

	if err := r.Update(ctx, &cm, &client.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

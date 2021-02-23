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
	"github.com/Skyscanner/argocd-progressive-rollout/internal/utils"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
)

// ProgressiveRolloutReconciler reconciles a ProgressiveRollout object
type ProgressiveRolloutReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=deployment.skyscanner.net,resources=progressiverollouts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deployment.skyscanner.net,resources=progressiverollouts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups="argoproj.io",resources=applications/status,verbs=get;list;watch

func (r *ProgressiveRolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("progressiverollout", req.NamespacedName)

	// Get the ProgressiveRollout object
	pr := deploymentskyscannernetv1alpha1.ProgressiveRollout{}
	if err := r.Get(ctx, req.NamespacedName, &pr); err != nil {
		log.Error(err, "unable to fetch ProgressiveRollout")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Always log the ApplicationSet owner
	log = r.Log.WithValues("applicationset", pr.Spec.SourceRef.Name)

	for _, stage := range pr.Spec.Stages {
		log = r.Log.WithValues("stage", stage.Name)

		targets, err := r.GetTargetClusters(stage.Targets.Clusters.Selector)
		if err != nil {
			log.Error(err, "unable to fetch targets")
			return ctrl.Result{}, err
		}
		r.Log.V(1).Info("targets selected", "targets", targets.Items)
		r.Log.Info("stage completed")
	}

	log.Info("all stages completed")

	// Rollout completed
	completed := pr.NewStatusCondition(deploymentskyscannernetv1alpha1.CompletedCondition, metav1.ConditionTrue, deploymentskyscannernetv1alpha1.StagesCompleteReason, "All stages completed")
	apimeta.SetStatusCondition(pr.GetStatusConditions(), completed)
	if err := r.Client.Status().Update(ctx, &pr); err != nil {
		r.Log.Error(err, "failed to update object status")
		return ctrl.Result{}, err
	}
	r.Log.Info("rollout completed")
	return ctrl.Result{}, nil
}

func (r *ProgressiveRolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deploymentskyscannernetv1alpha1.ProgressiveRollout{}).
		Watches(
			&source.Kind{Type: &argov1alpha1.Application{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForApplicationChange)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(r.requestsForSecretChange),
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Complete(r)
}

func (r *ProgressiveRolloutReconciler) requestsForApplicationChange(o client.Object) []reconcile.Request {

	/*
		We trigger a Progressive Rollout reconciliation loop on an Application event if:
		- the Application owner is referenced by a ProgressiveRollout object
	*/

	var requests []reconcile.Request
	var list deploymentskyscannernetv1alpha1.ProgressiveRolloutList
	ctx := context.Background()

	app, ok := o.(*argov1alpha1.Application)
	if !ok {
		err := fmt.Errorf("expected application, got %T", o)
		r.Log.Error(err, "failed to convert object to application")
		return nil
	}

	if err := r.List(ctx, &list); err != nil {
		r.Log.Error(err, "failed to list ProgressiveRollout")
		return nil
	}

	for _, pr := range list.Items {
		if pr.IsOwnedBy(app.GetOwnerReferences()) {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pr.Namespace,
				Name:      pr.Name,
			}})
		}
	}

	return requests
}

func (r *ProgressiveRolloutReconciler) requestsForSecretChange(o client.Object) []reconcile.Request {

	/*
		We trigger a Progressive Rollout reconciliation loop on a Secret event if:
		- the Secret is an ArgoCD cluster, AND
		- there is an Application targeting that secret/cluster, AND
		- that Application owner is referenced by a ProgressiveRollout object
	*/

	var requests []reconcile.Request
	var prList deploymentskyscannernetv1alpha1.ProgressiveRolloutList
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
		r.Log.Error(err, "failed to list ProgressiveRollout")
		return nil
	}
	if err := r.List(ctx, &appList); err != nil {
		r.Log.Error(err, "failed to list Application")
		return nil
	}

	for _, pr := range prList.Items {
		for _, app := range appList.Items {
			if app.Spec.Destination.Server == string(s.Data["server"]) && pr.IsOwnedBy(app.GetOwnerReferences()) {
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

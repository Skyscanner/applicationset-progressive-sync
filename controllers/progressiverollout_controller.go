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
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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

func (r *ProgressiveRolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("progressiverollout", req.NamespacedName)

	// Get the ProgressiveRollout object
	pr := deploymentskyscannernetv1alpha1.ProgressiveRollout{}
	if err := r.Get(ctx, req.NamespacedName, &pr); err != nil {
		log.Error(err, "unable to fetch ProgressiveRollout", "object", pr.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Rollout completed
	completed := pr.NewStatusCondition(deploymentskyscannernetv1alpha1.CompletedCondition, metav1.ConditionTrue, deploymentskyscannernetv1alpha1.StagesCompleteReason, "All stages completed")
	apimeta.SetStatusCondition(pr.GetStatusConditions(), completed)
	if err := r.Client.Status().Update(ctx, &pr); err != nil {
		r.Log.V(1).Info("failed to update object status", "name", pr.Name, "namespace", pr.Namespace)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ProgressiveRolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deploymentskyscannernetv1alpha1.ProgressiveRollout{}).
		Watches(
			&source.Kind{Type: &argov1alpha1.Application{}},
			handler.EnqueueRequestsFromMapFunc(r.requestsForApplicationChange)).
		Watches(
			&source.Kind{Type: &corev1.Secret{}}, 			handler.EnqueueRequestsFromMapFunc(r.requestsForSecretChange)).
		Complete(r)
}

func (r *ProgressiveRolloutReconciler) requestsForApplicationChange(o client.Object) []reconcile.Request {
	var requests []reconcile.Request
	var list deploymentskyscannernetv1alpha1.ProgressiveRolloutList
	ctx := context.Background()

	if err := r.List(ctx, &list); err != nil {
		r.Log.Error(err,"failed to list ProgressiveRollout")
		return nil
	}

	for _, pr := range list.Items{
		for _, owner := range o.GetOwnerReferences() {
			if owner.Kind == pr.Spec.SourceRef.Kind && owner.APIVersion == *pr.Spec.SourceRef.APIGroup && owner.Name == pr.Spec.SourceRef.Name {
				// The Application is owned by an ApplicationSet
				// referenced in the ProgressiveRollout spec
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Namespace: pr.Namespace,
					Name:      pr.Name,
				}})
			}
		}
	}

	return requests
}

func (r *ProgressiveRolloutReconciler) requestsForSecretChange(o client.Object) []reconcile.Request {
	var requests []reconcile.Request

	requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
		Namespace: o.GetNamespace(),
		Name:      o.GetName(),
	}})
	return requests
}

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

package v1alpha1

import (
	"github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const ProgressiveSyncFinalizer = "finalizers.argoproj.skyscanner.net"

// ProgressiveSyncSpec defines the desired state of ProgressiveSync
type ProgressiveSyncSpec struct {
	// SourceRef defines the resource, example an ApplicationSet, which owns ArgoCD Applications
	//+kubebuilder:validation:Required
	SourceRef corev1.TypedLocalObjectReference `json:"sourceRef"`

	// Stages defines a list of Progressive Rollout stages
	//+kubebuilder:validation:Optional
	Stages []Stage `json:"stages,omitempty"`
}

// Stage defines a rollout stage
type Stage struct {
	// Name is a human friendly name for the stage
	//+kubebuilder:validation:Required
	Name string `json:"name"`

	// MaxParallel is how many selected targets to update in parallel
	//+kubebuilder:validation:Minimum:1
	MaxParallel intstr.IntOrString `json:"maxParallel"`

	// MaxTargets is the maximum number of selected targets to update
	//+kubebuilder:validation:Minimum:1
	MaxTargets intstr.IntOrString `json:"maxTargets"`

	// Targets is the targets to update in the stage
	//+kubebuilder:validation:Optional
	Targets Targets `json:"targets,omitempty"`
}

// Targets defines the targets of the progressive sync operation
type Targets struct {
	// Clusters is the a cluster type of targets
	//+kubebuilder:validation:Optional
	Clusters Clusters `json:"clusters"`
}

// Clusters defines a target of type clusters
type Clusters struct {
	// Selector is a label selector to get the clusters for the update
	//+kubebuilder:validation:Required
	Selector metav1.LabelSelector `json:"selector"`
}

type StageStatus string

const (
	StageStatusCompleted string = "StageCompleted"

	StageStatusProgressing string = "StageProgressing"

	StageStatusFailed string = "StageFailed"
)

// ProgressiveSyncStatus defines the observed state of ProgressiveSync
type ProgressiveSyncStatus struct {
	// ObservedGeneration is the last observed generation.
	// +kubebuilder:validation:Optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the condition for the ProgressiveSync.
	// +kubebuilder:validation:Optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastSyncedStage is the name of the last synced stage.
	// +kubebuilder:validation:Optional
	LastSyncedStage string `json:"lastSyncedStage,omitempty"`

	// LastSyncedStageStatus is the status of the last synced stage.
	// +kubebuilder:validation:Optional
	LastSyncedStageStatus StageStatus `json:"lastSyncedStageStatus,omitempty"`
}

// Owns returns true if the ProgressiveSync object has a reference to one of the owners.
func (in *ProgressiveSync) Owns(owners []metav1.OwnerReference) bool {
	for _, owner := range owners {
		if owner.Kind == in.Spec.SourceRef.Kind && owner.APIVersion == *in.Spec.SourceRef.APIGroup && owner.Name == in.Spec.SourceRef.Name {
			return true
		}
	}
	return false
}

// ProgressiveSyncProgressing resets any previous information and registers progress toward
// reconciling the given ProgressiveSync by setting the meta.ReadyCondition to
// 'Unknown' for meta.ProgressingReason.
func ProgressiveSyncProgressing(ps ProgressiveSync) ProgressiveSync {
	ps.Status.Conditions = []metav1.Condition{}
	meta.SetResourceCondition(&ps, meta.ReadyCondition, metav1.ConditionUnknown, meta.ProgressingReason,
		"Reconciliation in progress")

	return ps
}

// ProgressiveSyncNotReady registers a failed reconciliation of the given ProgressiveSync.
func ProgressiveSyncNotReady(ps ProgressiveSync, reason, message string) ProgressiveSync {
	meta.SetResourceCondition(&ps, meta.ReadyCondition, metav1.ConditionFalse, reason, message)

	return ps
}

// ProgressiveSyncReady registers a successful reconciliation of the given ProgressiveSync.
func ProgressiveSyncReady(ps ProgressiveSync) ProgressiveSync {
	meta.SetResourceCondition(&ps, meta.ReadyCondition, metav1.ConditionTrue, meta.ReconciliationSucceededReason,
		"Progressive sync reconciliation succeeded")

	return ps
}

// GetStatusConditions returns a pointer to the Status.Conditions slice
func (in *ProgressiveSync) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// ProgressiveSync is the Schema for the progressivesyncs API
type ProgressiveSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProgressiveSyncSpec   `json:"spec,omitempty"`
	Status ProgressiveSyncStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProgressiveSyncList contains a list of ProgressiveSync
type ProgressiveSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProgressiveSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProgressiveSync{}, &ProgressiveSyncList{})
}

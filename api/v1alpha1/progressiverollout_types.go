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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ProgressiveRolloutSpec defines the desired state of ProgressiveRollout
type ProgressiveRolloutSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SourceRef references the resource, example an ApplicationSet, which owns ArgoCD Applications
	//+kubebuilder:validation:Required
	SourceRef corev1.TypedLocalObjectReference `json:"sourceRef"`
	// Stages reference a list of Progressive Rollout stages
	//+kubebuilder:validation:Required
	Stages []*ProgressiveRolloutStage `json:"stages"`
}

// ProgressiveRolloutStage defines a rollout stage
type ProgressiveRolloutStage struct {
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
	//+kubebuilder:validation:Required
	Targets ProgressiveRolloutTargets `json:"targets"`
}

type ProgressiveRolloutTargets struct {
	// Selector is a label selector to get the clusters for the update
	//+kubebuilder:validation:Required
	Selector metav1.LabelSelector `json:"selector"`
}

// ProgressiveRolloutStatus defines the observed state of ProgressiveRollout
type ProgressiveRolloutStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// ProgressiveRollout is the Schema for the progressiverollouts API
type ProgressiveRollout struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProgressiveRolloutSpec   `json:"spec,omitempty"`
	Status ProgressiveRolloutStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProgressiveRolloutList contains a list of ProgressiveRollout
type ProgressiveRolloutList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProgressiveRollout `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ProgressiveRollout{}, &ProgressiveRolloutList{})
}

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
	//+kubebuilder:validation:Optional
	Stages []*ProgressiveRolloutStage `json:"stages,omitempty"`
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
	//+kubebuilder:validation:Optional
	Targets ProgressiveRolloutTargets `json:"targets,omitempty"`
}

// ProgressiveRolloutTargets defines the target of the Progressive Rollout
type ProgressiveRolloutTargets struct {
	// Clusters is the a cluster type of targets
	//+kubebuilder:validation:Optional
	Clusters Cluster `json:"clusters"`
}

// Cluster defines a target of type cluster
type Cluster struct {
	// Selector is a label selector to get the clusters for the update
	//+kubebuilder:validation:Required
	Selector metav1.LabelSelector `json:"selector"`
}

// ProgressiveRolloutStatus defines the observed state of ProgressiveRollout
type ProgressiveRolloutStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []ProgressiveRolloutCondition `json:"conditions,omitempty"`
}

// ProgressiveRolloutCondition defines a ProgressiveRollout condition
type ProgressiveRolloutCondition struct {
	// Type of Progressive Rollout condition
	Type ProgressiveRolloutConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown
	Status metav1.ConditionStatus `json:"status"`
	// Reason is a one-word CamelCase reason for the condition's last transition
	Reason string `json:"reason,omitempty"`
	// LastTransitionTime of this condition
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

type ProgressiveRolloutConditionType string

const CompletedType ProgressiveRolloutConditionType = "Completed"

// NewCondition creates a new ProgressiveRolloutCondition
func (prs *ProgressiveRolloutStatus) NewCondition(ct ProgressiveRolloutConditionType, s metav1.ConditionStatus, r string) ProgressiveRolloutCondition {
	return ProgressiveRolloutCondition{
		Type:               ct,
		Status:             s,
		Reason:             r,
		LastTransitionTime: metav1.Now(),
	}
}

// GetCondition returns a ProgressiveRolloutCondition with the provided type if it exists, nil otherwise.
func (prs *ProgressiveRolloutStatus) GetCondition(ct ProgressiveRolloutConditionType) *ProgressiveRolloutCondition {
	for _, c := range prs.Conditions {
		if c.Type == ct {
			return &c
		}
	}
	return nil
}

// SetCondition adds/replaces the given condition. If the condition already exists, has the same status and reason, we don't update it.
func (prs *ProgressiveRolloutStatus) SetCondition(c *ProgressiveRolloutCondition) {
	currentCondition := prs.GetCondition(c.Type)
	if currentCondition != nil && currentCondition.Status == c.Status && currentCondition.Reason == c.Reason {
		return
	}
	prs.Conditions = append(prs.filterOutCondition(c.Type), *c)
}

// RemoveCondition removes the condition with the provided type from the status.
func (prs *ProgressiveRolloutStatus) RemoveCondition(ct ProgressiveRolloutConditionType) {
	prs.Conditions = prs.filterOutCondition(ct)
}

// filterOutCondition returns a new slice of conditions without conditions with the provided type.
func (prs *ProgressiveRolloutStatus) filterOutCondition(ct ProgressiveRolloutConditionType) []ProgressiveRolloutCondition {
	var newConditions []ProgressiveRolloutCondition
	for _, c := range prs.Conditions {
		if c.Type == ct {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
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

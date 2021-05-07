package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StageStatusPhase defines the observed stage phase
// +kubebuilder:validation:Enum={Progressing,Succeeded,Failed,Unknown}
type StageStatusPhase string

const (
	PhaseProgressing StageStatusPhase = "Progressing"
	PhaseSucceeded   StageStatusPhase = "Succeeded"
	PhaseFailed      StageStatusPhase = "Failed"
	PhaseUnknown     StageStatusPhase = "Unknown"
)

// StageStatus defines the observed stage status
type StageStatus struct {
	Name       string           `json:"name"`
	Phase      StageStatusPhase `json:"stage,omitempty"`
	Message    string           `json:"message,omitempty"`
	StartedAt  *metav1.Time     `json:"startedAt,omitempty"`
	FinishedAt *metav1.Time     `json:"finishedAt,omitempty"`
}

// GetStageStatus returns the Status.Stages slice
func (in *ProgressiveRollout) GetStageStatus() []StageStatus {
	return in.Status.Stages
}

// NewStageStatus adds a new StageStatus
func NewStageStatus(name, message string, phase StageStatusPhase) StageStatus {
	return StageStatus{
		Name:       name,
		Phase:      phase,
		Message:    message,
		StartedAt:  nil,
		FinishedAt: nil,
	}
}

// FindStageStatus finds the given stage by name in stage status.
func FindStageStatus(ss []StageStatus, statusName string) *StageStatus {
	for i, s := range ss {
		if s.Name == statusName {
			return &ss[i]
		}
	}

	return nil
}

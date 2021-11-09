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
	Phase      StageStatusPhase `json:"phase,omitempty"`
	Message    string           `json:"message,omitempty"`
	StartedAt  *metav1.Time     `json:"startedAt,omitempty"`
	FinishedAt *metav1.Time     `json:"finishedAt,omitempty"`
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

// SetStageStatus sets the corresponding StageStatus in stageStatus to newStatus
// - If a stage doesn't exist, it will be added to StageStatus slice
// - If a stage already exists it will be updated
func (in *ProgressiveSync) SetStageStatus(newStatus StageStatus, updateTime *metav1.Time) {
	// If StartedAt is not set and the stage is in progress, assign StartedAt
	if newStatus.Phase == PhaseProgressing && newStatus.StartedAt.IsZero() {
		newStatus.StartedAt = updateTime
	}
	// If the stage is not progressing it is either completed or failed.
	// If FinishedAt is not set we assign it.
	if (newStatus.Phase == PhaseFailed || newStatus.Phase == PhaseSucceeded) && newStatus.FinishedAt.IsZero() {
		newStatus.FinishedAt = updateTime
	}

	// Get the status if it already exists
	existingStatus := FindStageStatus(in.Status.Stages, newStatus.Name)

	if existingStatus == nil {
		in.Status.Stages = append(in.Status.Stages, newStatus)
		return
	}

	existingStatus.Phase = newStatus.Phase
	existingStatus.Message = newStatus.Message
	existingStatus.StartedAt = newStatus.StartedAt
	existingStatus.FinishedAt = newStatus.FinishedAt
}

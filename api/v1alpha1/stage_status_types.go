package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// StageStatusPhase defines the observed stage phase
// +kubebuilder:validation:Enum={Progressing,Succeeded,Failed}
type StageStatusPhase string

const (
	PhaseProgressing StageStatusPhase = "Progressing"
	PhaseSucceeded   StageStatusPhase = "Succeeded"
	PhaseFailed      StageStatusPhase = "Failed"
)

// StageStatus defines the observed stage status
type StageStatus struct {
	Name       string           `json:"name"`
	Phase      StageStatusPhase `json:"stage,omitempty"`
	Message    string           `json:"message,omitempty"`
	StartedAt  *metav1.Time     `json:"startedAt,omitempty"`
	FinishedAt *metav1.Time     `json:"finishedAt,omitempty"`
}

// GetStageStatus returns a pointer to the Status.Stages slice
func (in *ProgressiveRollout) GetStageStatus() *[]StageStatus {
	return &in.Status.Stages
}

// NewStageStatus adds a new StageStatus
func (in *ProgressiveRollout) NewStageStatus(name, message string, phase StageStatusPhase) StageStatus {
	return StageStatus{
		Name:       name,
		Phase:      phase,
		Message:    message,
		StartedAt:  nil,
		FinishedAt: nil,
	}
}

// SetStageStatus sets the corresponding StageStatus in stageStatus to newStatus
// - If a stage doesn't exist, it will be added to StageStatus slice
// - If a stage already exists it will be updated
func SetStageStatus(stageStatus *[]StageStatus, newStatus StageStatus) {

	nowTime := metav1.NewTime(time.Now())

	// If StartedAt is not set and the stage is in progress, assign StartedAt
	if newStatus.Phase == PhaseProgressing && newStatus.StartedAt.IsZero() {
		newStatus.StartedAt = &nowTime
	}
	// If the stage is not progressing it is either completed of failed.
	// If FinishedAt is not set we assign it.
	if newStatus.Phase != PhaseProgressing && newStatus.FinishedAt.IsZero() {
		newStatus.FinishedAt = &nowTime
	}

	// Get the status if it already exists
	existingStatus := FindStageStatus(*stageStatus, newStatus.Name)

	if existingStatus == nil {
		*stageStatus = append(*stageStatus, newStatus)
		return
	}

	existingStatus.Phase = newStatus.Phase
	existingStatus.Message = newStatus.Message
	existingStatus.StartedAt = newStatus.StartedAt
	existingStatus.FinishedAt = newStatus.FinishedAt
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

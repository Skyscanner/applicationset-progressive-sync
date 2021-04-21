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
	Targets    []string         `json:"targets,omitempty"`
	Syncing    []string         `json:"syncing,omitempty"`
	Requeued   []string         `json:"requeued,omitempty"`
	Failed     []string         `json:"failed,omitempty"`
	Completed  []string         `json:"completed,omitempty"`
	StartedAt  metav1.Time      `json:"startedAt,omitempty"`
	FinishedAt metav1.Time      `json:"finishedAt,omitempty"`
}

// GetStageStatus returns a pointer to the Status.Stages slice
func (in *ProgressiveRollout) GetStageStatus() *[]StageStatus {
	return &in.Status.Stages
}

// NewStageStatus adds a new StageStatus
func (in *ProgressiveRollout) NewStageStatus(n, m string, p StageStatusPhase, t, s, r, f, c []string) StageStatus {
	return StageStatus{
		Name:       n,
		Phase:      p,
		Message:    m,
		Targets:    t,
		Syncing:    s,
		Requeued:   r,
		Failed:     f,
		Completed:  c,
		StartedAt:  metav1.Time{},
		FinishedAt: metav1.Time{},
	}
}

// SetStageStatus sets the corresponding StageStatus in stageStatus to newStatus
func SetStageStatus(stageStatus *[]StageStatus, newStatus StageStatus) {
	existingStatus := FindStageStatus(*stageStatus, newStatus.Name)

	if existingStatus == nil {
		*stageStatus = append(*stageStatus, newStatus)
		return
	}

	existingStatus.Phase = newStatus.Phase
	existingStatus.Message = newStatus.Message
	existingStatus.Targets = newStatus.Targets
	existingStatus.Syncing = newStatus.Syncing
	existingStatus.Requeued = newStatus.Requeued
	existingStatus.Failed = newStatus.Failed
	existingStatus.Completed = newStatus.Completed

	if existingStatus.Phase == PhaseProgressing && existingStatus.StartedAt.IsZero() {
		existingStatus.StartedAt = metav1.NewTime(time.Now())
	}
	if existingStatus.Phase != PhaseProgressing && existingStatus.FinishedAt.IsZero() {
		existingStatus.FinishedAt = metav1.NewTime(time.Now())
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

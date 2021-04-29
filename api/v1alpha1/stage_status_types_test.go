package v1alpha1

import (
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestFindStageStatus(t *testing.T) {
	testCases := []struct {
		name        string
		statusName  string
		stageStatus []StageStatus
		expected    *StageStatus
	}{
		{
			name:        "stage missing",
			statusName:  "stage 1",
			stageStatus: []StageStatus{},
			expected:    nil,
		},
		{
			name:       "stage present",
			statusName: "stage 1",
			stageStatus: []StageStatus{{
				Name: "stage 1",
			}},
			expected: &StageStatus{
				Name: "stage 1",
			},
		},
		{
			name:       "stage present with multiple stages",
			statusName: "stage 2",
			stageStatus: []StageStatus{{
				Name: "stage 1",
			}, {
				Name: "stage 2",
			}},
			expected: &StageStatus{
				Name: "stage 2",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := FindStageStatus(testCase.stageStatus, testCase.statusName)
			g := NewWithT(t)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}

}

func TestSetStageStatus(t *testing.T) {
	pastTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	mockTime := metav1.NewTime(time.Now())

	testCases := []struct {
		name        string
		stageStatus *[]StageStatus
		newStatus   StageStatus
		expected    *[]StageStatus
	}{
		{
			name:        "add stage to empty list",
			stageStatus: &[]StageStatus{},
			newStatus: StageStatus{
				Name:  "stage 1",
				Phase: PhaseProgressing,
			},
			expected: &[]StageStatus{{
				Name:       "stage 1",
				Phase:      PhaseProgressing,
				Message:    "",
				StartedAt:  &mockTime,
				FinishedAt: nil,
			}},
		}, {
			name: "add stage to list",
			stageStatus: &[]StageStatus{{
				Name:      "stage 1",
				Phase:     PhaseProgressing,
				StartedAt: &pastTime,
			}},
			newStatus: StageStatus{
				Name:  "stage 2",
				Phase: PhaseProgressing,
			},
			expected: &[]StageStatus{{
				Name:       "stage 1",
				Phase:      PhaseProgressing,
				Message:    "",
				StartedAt:  &pastTime,
				FinishedAt: nil,
			}, {
				Name:       "stage 2",
				Phase:      PhaseProgressing,
				Message:    "",
				StartedAt:  &mockTime,
				FinishedAt: nil,
			}},
		}, {
			name: "update stage in list",
			stageStatus: &[]StageStatus{{
				Name:      "stage 1",
				Phase:     PhaseProgressing,
				StartedAt: &pastTime,
			}, {
				Name:      "stage 2",
				Phase:     PhaseProgressing,
				Message:   "old message",
				StartedAt: &pastTime,
			}},
			newStatus: StageStatus{
				Name:      "stage 2",
				Phase:     PhaseSucceeded,
				Message:   "new message",
				StartedAt: &pastTime,
			},
			expected: &[]StageStatus{{
				Name:       "stage 1",
				Phase:      PhaseProgressing,
				Message:    "",
				StartedAt:  &pastTime,
				FinishedAt: nil,
			}, {
				Name:       "stage 2",
				Phase:      PhaseSucceeded,
				Message:    "new message",
				StartedAt:  &pastTime,
				FinishedAt: &mockTime,
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			SetStageStatus(testCase.stageStatus, testCase.newStatus, &mockTime)
			g := NewWithT(t)
			g.Expect(testCase.stageStatus).To(Equal(testCase.expected))
		})
	}
}

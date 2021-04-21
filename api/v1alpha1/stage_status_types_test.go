package v1alpha1

import (
	. "github.com/onsi/gomega"
	"testing"
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := FindStageStatus(testCase.stageStatus, testCase.statusName)
			g := NewWithT(t)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}

}

package scheduler

import (
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	"github.com/Skyscanner/argocd-progressive-rollout/internal/utils"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func TestScheduler(t *testing.T) {
	namespace := "default"
	testCases := []struct {
		apps     []argov1alpha1.Application
		stage    deploymentskyscannernetv1alpha1.ProgressiveRolloutStage
		expected []string
	}{
		// 3 Applications:
		//  - OutOfSync 3
		//  - syncedInCurrentStage 0
		//  - Progressing 0
		// Stage:
		//  - maxTargets: 2
		//  - maxParallel: 2
		// The scheduler should return 2 applications
		{
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{
				Name:        "",
				MaxParallel: intstr.IntOrString{IntVal: 2},
				MaxTargets:  intstr.IntOrString{IntVal: 2},
				Targets:     deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{},
			},
			expected: []string{
				"app-one", "app-three",
			},
		},
		// 5 Applications:
		//  - OutOfSync 3
		//  - syncedInCurrentStage 1
		//  - Progressing 1
		// Stage:
		//  - maxTargets: 5
		//  - maxParallel: 2
		// The scheduler should return 1 application
		{
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app-four",
						Namespace:   namespace,
						Annotations: map[string]string{utils.ProgressiveRolloutSyncedAtStageKey: "test-5"},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app-five",
						Namespace:   namespace,
						Annotations: map[string]string{utils.ProgressiveRolloutSyncedAtStageKey: "test-5"},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
						Health: argov1alpha1.HealthStatus{Status: health.HealthStatusProgressing},
					},
				},
			},
			stage: deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{
				Name:        "test-5",
				MaxParallel: intstr.IntOrString{IntVal: 2},
				MaxTargets:  intstr.IntOrString{IntVal: 5},
				Targets:     deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{},
			},
			expected: []string{
				"app-one",
			},
		},
		// 5 Applications:
		//  - OutOfSync 5
		//  - syncedInCurrentStage 0
		//  - Progressing 0
		// Stage:
		//  - maxTargets: 50%
		//  - maxParallel: 100%
		// The scheduler should return 2 applications
		{
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{
				Name:        "test-5",
				MaxParallel: intstr.IntOrString{IntVal: 2},
				MaxTargets:  intstr.IntOrString{IntVal: 3},
				Targets:     deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{},
			},
			expected: []string{
				"app-five", "app-four",
			},
		},
		// 5 Applications:
		//  - OutOfSync 2
		//  - syncedInCurrentStage 3
		//  - Progressing 0
		// Stage:
		//  - maxTargets: 3
		//  - maxParallel: 1
		// The scheduler should return 0 applications
		{
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: namespace,
						Annotations: map[string]string{
							utils.ProgressiveRolloutSyncedAtStageKey: "test-5",
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: namespace,
						Annotations: map[string]string{
							utils.ProgressiveRolloutSyncedAtStageKey: "test-5",
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: namespace,
						Annotations: map[string]string{
							utils.ProgressiveRolloutSyncedAtStageKey: "test-5",
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{
				Name:        "test-5",
				MaxParallel: intstr.IntOrString{IntVal: 1},
				MaxTargets:  intstr.IntOrString{IntVal: 3},
				Targets:     deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{},
			},
			expected: nil,
		},
		// 1 Applications:
		//  - OutOfSync 1
		//  - syncedInCurrentStage 0
		//  - Progressing 0
		// Stage:
		//  - maxTargets: 1
		//  - maxParallel: 1
		// The scheduler should return 1 applications
		{
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{
				Name:        "",
				MaxParallel: intstr.IntOrString{IntVal: 1},
				MaxTargets:  intstr.IntOrString{IntVal: 1},
				Targets:     deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{},
			},
			expected: []string{
				"app-one",
			},
		},
		// 5 Applications:
		//  - OutOfSync 5
		//  - syncedInCurrentStage 0
		//  - Progressing 0
		// Stage:
		//  - maxTargets: 3
		//  - maxParallel: 3
		// The scheduler should return 3 applications
		{
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{
				Name:        "test-5",
				MaxParallel: intstr.IntOrString{IntVal: 3},
				MaxTargets:  intstr.IntOrString{IntVal: 3},
				Targets:     deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{},
			},
			expected: []string{
				"app-five", "app-four", "app-one",
			},
		},
		// 0 Applications
		// Stage:
		//  - maxTargets: 3
		//  - maxParallel: 3
		// The scheduler should return 0 applications
		{
			apps: nil,
			stage: deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{
				Name:        "test-5",
				MaxParallel: intstr.IntOrString{IntVal: 3},
				MaxTargets:  intstr.IntOrString{IntVal: 3},
				Targets:     deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{},
			},
			expected: nil,
		},
	}

	for _, testCase := range testCases {
		got := Scheduler(testCase.apps, testCase.stage)
		g := NewWithT(t)
		g.Expect(got).To(Equal(testCase.expected))
	}
}

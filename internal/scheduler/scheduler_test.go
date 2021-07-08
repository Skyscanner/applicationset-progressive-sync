package scheduler

import (
	"testing"

	"github.com/go-logr/logr"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/utils"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	SchedulerTestNamespace = "test-scheduler"
	StageName              = "stage"
)

func TestScheduler(t *testing.T) {
	testCases := []struct {
		name          string
		apps          []argov1alpha1.Application
		stage         syncv1alpha1.ProgressiveSyncStage
		syncedAtStage map[string]string
		expected      []argov1alpha1.Application
	}{
		{
			name: "Applications: outOfSync 3, syncedInCurrentStage 0, progressing 0, | Stage: maxTargets 2, maxParallel 2 | Expected: scheduled 2",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
		},
		{
			name: "Applications: outOfSync 3, syncedInCurrentStage 1, progressing 1, | Stage: maxTargets 5, maxParallel 2 | Expected: scheduled 1",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace:   SchedulerTestNamespace,
						Annotations: map[string]string{utils.ProgressiveSyncSyncedAtStageKey: StageName},
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
						Namespace:   SchedulerTestNamespace,
						Annotations: map[string]string{utils.ProgressiveSyncSyncedAtStageKey: StageName},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
						Health: argov1alpha1.HealthStatus{Status: health.HealthStatusProgressing},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: map[string]string{
				"psName/app-four": StageName,
				"psName/app-five": StageName,
			},
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
		},
		{
			name: "Applications: outOfSync 5, syncedInCurrentStage 0, progressing 0, | Stage: maxTargets 3, maxParallel 2 | Expected: scheduled 2",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
		},
		{
			name: "Applications: outOfSync 5, syncedInCurrentStage 0, progressing 0, | Stage: maxTargets 50%, maxParallel 100% | Expected: scheduled 2",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("100%"),
				MaxTargets:  intstr.Parse("50%"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
		},
		{
			name: "Applications: outOfSync 2, syncedInCurrentStage 3, progressing 0, | Stage: maxTargets 3, maxParallel 1 | Expected: scheduled 0",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
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
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
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
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("1"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: map[string]string{
				"psName/app-two":   StageName,
				"psName/app-three": StageName,
				"psName/app-four":  StageName,
			},
			expected: nil,
		},
		{
			name: "Applications: outOfSync 1, syncedInCurrentStage 0, progressing 0, | Stage: maxTargets 1, maxParallel 1 | Expected: scheduled 1",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("1"),
				MaxTargets:  intstr.Parse("1"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
		},
		{
			name: "Applications: outOfSync 5, syncedInCurrentStage 0, progressing 0, | Stage: maxTargets 3, maxParallel 3 | Expected: scheduled 3",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("3"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
		},
		{
			name: "Applications: outOfSync 2, syncedInCurrentStage 0, progressing 0, | Stage: maxTargets 10%, maxParallel 10% | Expected: scheduled 1",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("10%"),
				MaxTargets:  intstr.Parse("10%"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
		},
		{
			name: "Applications: outOfSync 2, syncedInCurrentStage 0, progressing 0, syncedInPreviousStage 1 | Stage: maxTargets 2, maxParallel 2 | Expected: scheduled 2",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app-three",
						Namespace:   SchedulerTestNamespace,
						Annotations: map[string]string{utils.ProgressiveSyncSyncedAtStageKey: "previous-stage"},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("2"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: map[string]string{
				"psName/app-three": "previous-stage",
			},
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
		},
		{
			name: "Applications: outOfSync 0, syncedInCurrentStage 0, progressing 0, | Stage: maxTargets 3, maxParallel 3 | Expected: scheduled 0",
			apps: nil,
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("3"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected:      nil,
		},
		{
			name: "Applications: outOfSync 4, syncedInCurrentStage 2, progressing 1, syncedInPreviousStage 2 | Stage: maxTargets 3, maxParallel 3 | Expected: scheduled 2",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app-one",
						Namespace:   SchedulerTestNamespace,
						Annotations: map[string]string{utils.ProgressiveSyncSyncedAtStageKey: StageName},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app-two",
						Namespace:   SchedulerTestNamespace,
						Annotations: map[string]string{utils.ProgressiveSyncSyncedAtStageKey: StageName},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusProgressing,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-six",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app-seven",
						Namespace:   SchedulerTestNamespace,
						Annotations: map[string]string{utils.ProgressiveSyncSyncedAtStageKey: "previous-stage"},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "app-eight",
						Namespace:   SchedulerTestNamespace,
						Annotations: map[string]string{utils.ProgressiveSyncSyncedAtStageKey: "previous-stage"},
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("3"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: map[string]string{
				"psName/app-one":   StageName,
				"psName/app-two":   StageName,
				"psName/app-seven": "previous-stage",
				"psName/app-eight": "previous-stage",
			},
			expected: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: SchedulerTestNamespace,
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
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusProgressing,
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		log := logr.Discard()
		t.Run(testCase.name, func(t *testing.T) {
			utils.SortAppsByName(testCase.apps)
			got := Scheduler(log, testCase.apps, testCase.stage, testCase.syncedAtStage, "psName")
			g := NewWithT(t)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}
}

func TestIsStageFailed(t *testing.T) {
	testCases := []struct {
		name          string
		apps          []argov1alpha1.Application
		stage         syncv1alpha1.ProgressiveSyncStage
		syncedAtStage map[string]string
		expected      bool
	}{
		{
			name: "stage failed",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusUnknown,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusProgressing,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusMissing,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusSuspended,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeUnknown,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-six",
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: map[string]string{
				"psName/app-six": StageName,
			},
			expected: true,
		},
		{
			name: "stage not failed",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusUnknown,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusProgressing,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusMissing,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusSuspended,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected:      false,
		},
		{
			name: "stage not failed when apps is nil",
			apps: nil,
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := IsStageFailed(testCase.apps, testCase.stage, testCase.syncedAtStage, "psName")
			g := NewWithT(t)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}
}

func TestIsStageInProgress(t *testing.T) {
	testCases := []struct {
		name          string
		apps          []argov1alpha1.Application
		stage         syncv1alpha1.ProgressiveSyncStage
		syncedAtStage map[string]string
		expected      bool
	}{
		{
			name: "stage in progress",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusUnknown,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusMissing,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusSuspended,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-six",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusProgressing,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected:      true,
		},
		{
			name: "stage not in progress",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusUnknown,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusMissing,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-five",
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusSuspended,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: map[string]string{
				"psName/app-one":   StageName,
				"psName/app-two":   StageName,
				"psName/app-three": StageName,
				"psName/app-four":  StageName,
				"psName/app-five":  StageName,
			},
			expected: false,
		},
		{
			name: "stage not in progress when apps is nil",
			apps: nil,
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected:      false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := IsStageInProgress(testCase.apps, testCase.stage, testCase.syncedAtStage, "psName")
			g := NewWithT(t)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}
}

func TestIsStageComplete(t *testing.T) {
	testCases := []struct {
		name          string
		apps          []argov1alpha1.Application
		stage         syncv1alpha1.ProgressiveSyncStage
		syncedAtStage map[string]string
		expected      bool
	}{
		{
			name: "stage is complete",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: SchedulerTestNamespace,
						Annotations: map[string]string{
							utils.ProgressiveSyncSyncedAtStageKey: StageName,
						},
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("2"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: map[string]string{
				"psName/app-one": StageName,
				"psName/app-two": StageName,
			},
			expected: true,
		},
		{
			name: "stage is not complete",
			apps: []argov1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-one",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-two",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusDegraded,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-three",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusMissing,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeUnknown,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-four",
						Namespace: SchedulerTestNamespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusHealthy,
						},
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeSynced,
						},
					},
				},
			},
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected:      false,
		},
		{
			name: "stage is completed when apps is nil",
			apps: nil,
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			syncedAtStage: make(map[string]string),
			expected:      true,
		},
		{
			name: "stage is completed when apps is nil",
			apps: nil,
			stage: syncv1alpha1.ProgressiveSyncStage{
				Name:        StageName,
				MaxParallel: intstr.Parse("2"),
				MaxTargets:  intstr.Parse("3"),
				Targets:     syncv1alpha1.ProgressiveSyncTargets{},
			},
			expected: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := IsStageComplete(testCase.apps, testCase.stage, testCase.syncedAtStage, "psName")
			g := NewWithT(t)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}
}

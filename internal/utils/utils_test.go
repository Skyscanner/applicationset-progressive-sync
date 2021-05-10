package utils

import (
	"testing"

	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsArgoCDCluster(t *testing.T) {
	testCases := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{{
		name:     "correct secret label key and value",
		labels:   map[string]string{"foo": "bar", ArgoCDSecretTypeLabel: ArgoCDSecretTypeCluster, "key": "value"},
		expected: true,
	}, {
		name:     "correct secret label key but wrong value",
		labels:   map[string]string{"foo": "bar", ArgoCDSecretTypeLabel: "wrong-value", "key": "value"},
		expected: false,
	}, {
		name:     "wrong secret label key and value",
		labels:   map[string]string{"foo": "bar", "wrong-label": "wrong-value", "key": "value"},
		expected: false,
	}, {
		name:     "correct secret label value but wrong key ",
		labels:   map[string]string{"foo": "bar", "wrong-label": ArgoCDSecretTypeCluster, "key": "value"},
		expected: false,
	}, {
		name:     "missing secret label",
		labels:   map[string]string{"foo": "bar", "key": "value"},
		expected: false,
	}}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := IsArgoCDCluster(testCase.labels)
			g := NewWithT(t)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}
}

func TestSortSecretsByName(t *testing.T) {
	namespace := "default"
	testCase := struct {
		secretList *corev1.SecretList
		expected   *corev1.SecretList
	}{
		secretList: &corev1.SecretList{Items: []corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{Name: "clusterA", Namespace: namespace},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "clusterC", Namespace: namespace},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "clusterB", Namespace: namespace},
		}}},
		expected: &corev1.SecretList{Items: []corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{Name: "clusterA", Namespace: namespace},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "clusterB", Namespace: namespace},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "clusterC", Namespace: namespace},
		}}}}
	g := NewWithT(t)
	SortSecretsByName(testCase.secretList)
	g.Expect(testCase.secretList).Should(Equal(testCase.expected))
}

func TestSortAppsByName(t *testing.T) {
	namespace := "default"
	testCase := struct {
		apps     []argov1alpha1.Application
		expected []argov1alpha1.Application
	}{
		apps: []argov1alpha1.Application{{
			ObjectMeta: metav1.ObjectMeta{Name: "appA", Namespace: namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appC", Namespace: namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appB", Namespace: namespace}}},
		expected: []argov1alpha1.Application{{
			ObjectMeta: metav1.ObjectMeta{Name: "appA", Namespace: namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appB", Namespace: namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appC", Namespace: namespace}}},
	}

	g := NewWithT(t)
	SortAppsByName(testCase.apps)
	g.Expect(testCase.apps).Should(Equal(testCase.expected))
}

func TestGetSyncedAppsByStage(t *testing.T) {
	namespace := "default"
	stage := "test-stage"
	testCases := []struct {
		name     string
		apps     []argov1alpha1.Application
		stage    string
		expected []argov1alpha1.Application
	}{
		{
			name: "Correct annotation, stage, sync status, health status",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
					Annotations: map[string]string{
						ProgressiveRolloutSyncedAtStageKey: stage,
					}},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
					Health: argov1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					},
				},
			}},
			stage: stage,
			expected: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
					Annotations: map[string]string{
						ProgressiveRolloutSyncedAtStageKey: stage,
					}},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
					Health: argov1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					},
				},
			}},
		},
		{
			name: "Correct annotation, sync status, health status. Incorrect annotation value",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
					Annotations: map[string]string{
						ProgressiveRolloutSyncedAtStageKey: "wrong-stage",
					}},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
					Health: argov1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					},
				}},
			},
			stage:    stage,
			expected: nil,
		},
		{
			name: "Correct annotation, value, sync status. Incorrect health status",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
					Annotations: map[string]string{
						ProgressiveRolloutSyncedAtStageKey: stage,
					}},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
					Health: argov1alpha1.HealthStatus{
						Status: health.HealthStatusProgressing,
					},
				}},
			},
			stage:    stage,
			expected: nil,
		},
		{
			name: "Correct annotation, value and health status. Incorrect sync status",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
					Annotations: map[string]string{
						ProgressiveRolloutSyncedAtStageKey: stage,
					}},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeOutOfSync},
					Health: argov1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					}},
			},
			},
			stage:    stage,
			expected: nil,
		},
		{
			name: "Missing annotation",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
				},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
					Health: argov1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					}},
			},
			},
			stage:    stage,
			expected: nil,
		},
		{
			name: "2 Applications: 1 with correct annotation, stage, sync status, health status. 1 with incorrect data",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
					Annotations: map[string]string{
						ProgressiveRolloutSyncedAtStageKey: stage,
					}},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
					Health: argov1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					},
				},
			},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "appB",
						Namespace: namespace,
					},
					Status: argov1alpha1.ApplicationStatus{
						Sync: argov1alpha1.SyncStatus{
							Status: argov1alpha1.SyncStatusCodeOutOfSync,
						},
						Health: argov1alpha1.HealthStatus{
							Status: health.HealthStatusProgressing,
						},
					},
				}},
			stage: stage,
			expected: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
					Annotations: map[string]string{
						ProgressiveRolloutSyncedAtStageKey: stage,
					}},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
					Health: argov1alpha1.HealthStatus{
						Status: health.HealthStatusHealthy,
					},
				},
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			got := GetSyncedAppsByStage(testCase.apps, testCase.stage)
			g.Expect(got).Should(Equal(testCase.expected))
		})
	}
}

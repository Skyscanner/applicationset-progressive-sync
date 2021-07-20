package utils

import (
	"testing"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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
		labels:   map[string]string{"foo": "bar", consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster, "key": "value"},
		expected: true,
	}, {
		name:     "correct secret label key but wrong value",
		labels:   map[string]string{"foo": "bar", consts.ArgoCDSecretTypeLabel: "wrong-value", "key": "value"},
		expected: false,
	}, {
		name:     "wrong secret label key and value",
		labels:   map[string]string{"foo": "bar", "wrong-label": "wrong-value", "key": "value"},
		expected: false,
	}, {
		name:     "correct secret label value but wrong key ",
		labels:   map[string]string{"foo": "bar", "wrong-label": consts.ArgoCDSecretTypeCluster, "key": "value"},
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

type GetSyncedAppsByStageTestCase struct {
	name          string
	apps          []argov1alpha1.Application
	syncedAtStage map[string]syncv1alpha1.ProgressiveSyncStage
	expected      []argov1alpha1.Application
}

func TestGetSyncedAppsByStage(t *testing.T) {
	namespace := "default"
	stage := syncv1alpha1.ProgressiveSyncStage{
		Name: "test-stage",
	}
	wrongStage := syncv1alpha1.ProgressiveSyncStage{
		Name: "wrong-stage",
	}
	testCases := []GetSyncedAppsByStageTestCase{
		{
			name: "appA marked as synced, in the correct stage",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
				},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
				},
			}},
			syncedAtStage: map[string]syncv1alpha1.ProgressiveSyncStage{
				"appA": stage,
			},
			expected: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
				},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
				},
			}},
		},
		{
			name: "appA marked as synced, but in a previous stage",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
				},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
				}},
			},
			syncedAtStage: map[string]syncv1alpha1.ProgressiveSyncStage{
				"appA": wrongStage,
			},
			expected: nil,
		},
		{
			name: "appA marked as synced, but incorrect sync status",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
				},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeOutOfSync},
				}},
			},
			syncedAtStage: map[string]syncv1alpha1.ProgressiveSyncStage{
				"appA": stage,
			},
			expected: nil,
		},
		{
			name: "appA not marked as synced",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
				},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
				}},
			},
			syncedAtStage: make(map[string]syncv1alpha1.ProgressiveSyncStage),
			expected:      nil,
		},
		{
			name: "2 Applications: 1 correctly marked as synced, stage, sync status and 1 with incorrect data",
			apps: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
				},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
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
					},
				}},
			syncedAtStage: map[string]syncv1alpha1.ProgressiveSyncStage{
				"appA": stage,
			},
			expected: []argov1alpha1.Application{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appA",
					Namespace: namespace,
				},
				Status: argov1alpha1.ApplicationStatus{
					Sync: argov1alpha1.SyncStatus{
						Status: argov1alpha1.SyncStatusCodeSynced},
				},
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			pss := PopulateState(testCase)
			got := GetSyncedAppsByStage(testCase.apps, stage, pss)
			g.Expect(got).Should(Equal(testCase.expected))
		})
	}
}

func PopulateState(testCase GetSyncedAppsByStageTestCase) ProgressiveSyncState {
	pss, _ := NewProgressiveSyncManager().Get(testCase.name)
	for appName, stage := range testCase.syncedAtStage {
		var stageApp argov1alpha1.Application
		for i := 0; i < len(testCase.apps); i++ {
			if testCase.apps[i].Name == appName {
				stageApp = testCase.apps[i]
			}

		}
		pss.MarkAppAsSynced(stageApp, stage)
	}
	return pss
}

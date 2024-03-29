package utils

import (
	"testing"

	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	applicationset "github.com/argoproj-labs/applicationset/api/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const Namespace = "default"

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
		name:     "correct secret label value but wrong key",
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
	testCase := struct {
		secretList *corev1.SecretList
		expected   *corev1.SecretList
	}{
		secretList: &corev1.SecretList{Items: []corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{Name: "clusterA", Namespace: Namespace},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "clusterC", Namespace: Namespace},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "clusterB", Namespace: Namespace},
		}}},
		expected: &corev1.SecretList{Items: []corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{Name: "clusterA", Namespace: Namespace},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "clusterB", Namespace: Namespace},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "clusterC", Namespace: Namespace},
		}}}}
	g := NewWithT(t)
	SortSecretsByName(testCase.secretList)
	g.Expect(testCase.secretList).Should(Equal(testCase.expected))
}

func TestSortAppsByName(t *testing.T) {
	testCase := struct {
		apps     []argov1alpha1.Application
		expected []argov1alpha1.Application
	}{
		apps: []argov1alpha1.Application{{
			ObjectMeta: metav1.ObjectMeta{Name: "appA", Namespace: Namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appC", Namespace: Namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appB", Namespace: Namespace}}},
		expected: []argov1alpha1.Application{{
			ObjectMeta: metav1.ObjectMeta{Name: "appA", Namespace: Namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appB", Namespace: Namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appC", Namespace: Namespace}}},
	}

	g := NewWithT(t)
	SortAppsByName(testCase.apps)
	g.Expect(testCase.apps).Should(Equal(testCase.expected))
}

func TestHash(t *testing.T) {
	g := NewWithT(t)
	appSet := applicationset.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: Namespace,
		},
		Spec: applicationset.ApplicationSetSpec{
			Generators: []applicationset.ApplicationSetGenerator{},
		},
	}
	appSetHash := ComputeHash(appSet.Spec)
	g.Expect(appSetHash).NotTo(BeNil())

	appSet.Spec.SyncPolicy = &applicationset.ApplicationSetSyncPolicy{
		PreserveResourcesOnDeletion: true,
	}
	newAppSetHash := ComputeHash(appSet.Spec)
	g.Expect(newAppSetHash).NotTo(BeNil())

	g.Expect(appSetHash).NotTo(Equal(newAppSetHash))
}

func TestMin(t *testing.T) {
	testCases := []struct {
		name     string
		x        int
		y        int
		expected int
	}{
		{
			name:     "x smaller than y",
			x:        3,
			y:        5,
			expected: 3,
		},
		{
			name:     "y smaller than x",
			x:        7,
			y:        5,
			expected: 5,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			got := Min(testCase.x, testCase.y)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}
}

func TestHaveSameRevision(t *testing.T) {
	testCases := []struct {
		name     string
		apps     []argov1alpha1.Application
		expected bool
	}{
		{
			name: "same revision",
			apps: []argov1alpha1.Application{
				{ObjectMeta: metav1.ObjectMeta{Name: "appA", Namespace: Namespace},
					Status: argov1alpha1.ApplicationStatus{Sync: argov1alpha1.SyncStatus{Revision: "abcdef"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "appB", Namespace: Namespace},
					Status: argov1alpha1.ApplicationStatus{Sync: argov1alpha1.SyncStatus{Revision: "abcdef"}}},
			},
			expected: true,
		},
		{
			name: "different revision",
			apps: []argov1alpha1.Application{
				{ObjectMeta: metav1.ObjectMeta{Name: "appC", Namespace: Namespace},
					Status: argov1alpha1.ApplicationStatus{Sync: argov1alpha1.SyncStatus{Revision: "abcdef"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "appD", Namespace: Namespace},
					Status: argov1alpha1.ApplicationStatus{Sync: argov1alpha1.SyncStatus{Revision: "ghilmn"}}},
			},
			expected: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			g := NewWithT(t)
			got := HaveSameRevision(testCase.apps)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}
}

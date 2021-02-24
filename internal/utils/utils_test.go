package utils

import (
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestIsArgoCDCluster(t *testing.T) {
	testCases := []struct {
		labels   map[string]string
		expected bool
	}{{
		labels: map[string]string{"foo": "bar", ArgoCDSecretTypeLabel: ArgoCDSecretTypeCluster, "key": "value"}, expected: true,
	}, {
		labels:   map[string]string{"foo": "bar", ArgoCDSecretTypeLabel: "wrong-value", "key": "value"},
		expected: false,
	}, {
		labels:   map[string]string{"foo": "bar", "wrong-label": "wrong-value", "key": "value"},
		expected: false,
	}, {
		labels:   map[string]string{"foo": "bar", "wrong-label": ArgoCDSecretTypeCluster, "key": "value"},
		expected: false,
	}, {
		labels:   map[string]string{"foo": "bar", "key": "value"},
		expected: false,
	}}

	for _, testCase := range testCases {
		got := IsArgoCDCluster(testCase.labels)
		g := NewGomegaWithT(t)
		g.Expect(got).To(Equal(testCase.expected))
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
	g := NewGomegaWithT(t)
	SortSecretsByName(testCase.secretList)
	g.Expect(testCase.secretList).Should(Equal(testCase.expected))
}

func TestSortAppsByName(t *testing.T) {
	namespace := "default"
	testCase := struct {
		apps     *[]argov1alpha1.Application
		expected *[]argov1alpha1.Application
	}{
		apps: &[]argov1alpha1.Application{{
			ObjectMeta: metav1.ObjectMeta{Name: "appA", Namespace: namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appC", Namespace: namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appB", Namespace: namespace}}},
		expected: &[]argov1alpha1.Application{{
			ObjectMeta: metav1.ObjectMeta{Name: "appA", Namespace: namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appB", Namespace: namespace}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "appC", Namespace: namespace}}},
	}

	g := NewGomegaWithT(t)
	SortAppsByName(testCase.apps)
	g.Expect(testCase.apps).Should(Equal(testCase.expected))
}

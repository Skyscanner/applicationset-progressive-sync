package internal

import (
	. "github.com/onsi/gomega"
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

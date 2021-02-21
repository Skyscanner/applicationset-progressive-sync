package internal

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestIsArgoCDCluster(t *testing.T) {
	testCases := []struct {
		annotations map[string]string
		expected    bool
	}{{
		annotations: map[string]string{"foo": "bar", ArgoCDSecretTypeLabel: ArgoCDSecretTypeCluster, "key": "value"}, expected: true,
	}, {
		annotations: map[string]string{"foo": "bar", ArgoCDSecretTypeLabel: "wrong-value", "key": "value"},
		expected:    false,
	}, {
		annotations: map[string]string{"foo": "bar", "wrong-label": "wrong-value", "key": "value"},
		expected:    false,
	}, {
		annotations: map[string]string{"foo": "bar", "wrong-label": ArgoCDSecretTypeCluster, "key": "value"},
		expected:    false,
	}, {
		annotations: map[string]string{"foo": "bar", "key": "value"},
		expected:    false,
	}}

	for _, testCase := range testCases {
		got := IsArgoCDCluster(testCase.annotations)
		g := NewGomegaWithT(t)
		g.Expect(got).To(Equal(testCase.expected))
	}
}

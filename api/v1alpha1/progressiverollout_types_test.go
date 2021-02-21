package v1alpha1

import (
	"github.com/Skyscanner/argocd-progressive-rollout/internal"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestHasOwnerReference(t *testing.T) {
	testCases := []struct {
		ownerReferences []metav1.OwnerReference
		expected        bool
	}{{
		ownerReferences: []metav1.OwnerReference{{
			APIVersion: "fakeAPIVersion",
			Kind:       "fakeKind",
			Name:       "fakeName",
		}, {
			APIVersion: internal.AppSetAPIGroup,
			Kind:       internal.AppSetKind,
			Name:       "owner-app-set",
		}},
		expected: true,
	}, {
		ownerReferences: []metav1.OwnerReference{{
			APIVersion: "fakeAPIVersion",
			Kind:       "fakeKind",
			Name:       "fakeName",
		}},
		expected: false,
	},
	}
	ref := internal.AppSetAPIGroup
	pr := ProgressiveRollout{
		ObjectMeta: metav1.ObjectMeta{Name: "pr", Namespace: "namespace"},
		Spec: ProgressiveRolloutSpec{
			SourceRef: corev1.TypedLocalObjectReference{
				APIGroup: &ref,
				Kind:     internal.AppSetKind,
				Name:     "owner-app-set",
			},
			Stages: nil,
		}}

	for _, testCase := range testCases {
		got := pr.HasOwnerReference(testCase.ownerReferences)
		g := NewGomegaWithT(t)
		g.Expect(got).To(Equal(testCase.expected))
	}
}

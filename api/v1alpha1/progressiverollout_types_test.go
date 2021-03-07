package v1alpha1

import (
	"testing"

	"github.com/Skyscanner/argocd-progressive-rollout/internal/utils"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOwns(t *testing.T) {
	testCases := []struct {
		name            string
		ownerReferences []metav1.OwnerReference
		expected        bool
	}{{
		name: "owns",
		ownerReferences: []metav1.OwnerReference{{
			APIVersion: "fakeAPIVersion",
			Kind:       "fakeKind",
			Name:       "fakeName",
		}, {
			APIVersion: utils.AppSetAPIGroup,
			Kind:       utils.AppSetKind,
			Name:       "owner-app-set",
		}},
		expected: true,
	}, {
		name: "does not own",
		ownerReferences: []metav1.OwnerReference{{
			APIVersion: "fakeAPIVersion",
			Kind:       "fakeKind",
			Name:       "fakeName",
		}},
		expected: false,
	}}

	ref := utils.AppSetAPIGroup
	pr := ProgressiveRollout{
		ObjectMeta: metav1.ObjectMeta{Name: "pr", Namespace: "namespace"},
		Spec: ProgressiveRolloutSpec{
			SourceRef: corev1.TypedLocalObjectReference{
				APIGroup: &ref,
				Kind:     utils.AppSetKind,
				Name:     "owner-app-set",
			},
			Stages: nil,
		}}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := pr.Owns(testCase.ownerReferences)
			g := NewWithT(t)
			g.Expect(got).To(Equal(testCase.expected))
		})
	}
}

package v1alpha1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These tests are written in BDD-style using Ginkgo framework. Refer to
// http://onsi.github.io/ginkgo to learn more.

var _ = Describe("ProgressiveRollout Status", func() {

	Context("Status", func() {

		Describe("SetCondition", func() {
			It("should add a new conditions", func() {
				status := &ProgressiveRolloutStatus{}
				c := status.NewCondition(CompletedType, metav1.ConditionTrue, "Completed")
				status.SetCondition(&c)
				Expect(status.Conditions[0]).To(Equal(c))
				Expect(len(status.Conditions)).To(Equal(1))
			})
		})
	})
})

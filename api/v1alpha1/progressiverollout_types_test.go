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

			It("should replace a pre-existing conditions", func() {
				status := &ProgressiveRolloutStatus{}
				empty := status.NewCondition(CompletedType, metav1.ConditionTrue, "")
				status.SetCondition(&empty)
				nonEmpty := status.NewCondition(CompletedType, metav1.ConditionTrue, "Completed")
				status.SetCondition(&nonEmpty)
				Expect(status.Conditions[0]).To(Equal(nonEmpty))
				Expect(len(status.Conditions)).To(Equal(1))
			})

			It("should not update the condition if it already exists and has the same status and reason.", func() {
				status := &ProgressiveRolloutStatus{}
				first := status.NewCondition(CompletedType, metav1.ConditionFalse, "Failed")
				status.SetCondition(&first)
				second := status.NewCondition(CompletedType, metav1.ConditionFalse, "Failed")
				status.SetCondition(&second)
				Expect(status.Conditions[0]).To(Equal(first))
				Expect(len(status.Conditions)).To(Equal(1))
			})

		})
	})
})

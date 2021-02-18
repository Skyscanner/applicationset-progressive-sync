package controllers

import (
	"context"
	"fmt"
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"time"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 100
)

var _ = Describe("ProgressiveRollout Controller", func() {

	apiGroup := "argoproj.io/v1alpha1"
	testProgressiveRollout := "test-progressive-rollout"
	testApplicationSet := "test-application-set"
	testNamesapce := "test-namespace"
	ctx := context.Background()
	// See https://onsi.github.io/gomega#modifying-default-intervals
	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(interval)

	BeforeEach(func() {
		namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: testNamesapce},
		}
		err := k8sClient.Create(context.Background(), &namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")
	})

	// We should replace this test with better ones as we deploy the controller
	It("should reconcile", func() {
		pr := deploymentskyscannernetv1alpha1.ProgressiveRollout{
			ObjectMeta: metav1.ObjectMeta{Name: testProgressiveRollout, Namespace: testNamesapce},
			Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
				SourceRef: corev1.TypedLocalObjectReference{
					APIGroup: &apiGroup,
					Kind:     "ApplicationSet",
					Name:     testApplicationSet,
				},
				Stages: []deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{{
					Name:        "stage 1",
					MaxParallel: intstr.IntOrString{IntVal: 1},
					MaxTargets:  intstr.IntOrString{IntVal: 1},
					Targets: deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{Clusters: deploymentskyscannernetv1alpha1.Clusters{
						Selector: metav1.LabelSelector{MatchLabels: nil},
					}},
				}},
			},
		}
		Expect(k8sClient.Create(ctx, &pr)).To(Succeed())

		expected := pr.NewStatusCondition(deploymentskyscannernetv1alpha1.CompletedCondition, metav1.ConditionTrue, deploymentskyscannernetv1alpha1.StagesCompleteReason, "All stages completed")
		ExpectCondition(&pr, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))

	})
})

// statusString returns a formatted string with a condition status, reason and message
func statusString(status metav1.ConditionStatus, reason string, message string) string {
	return fmt.Sprintf("Status: %s, Reason: %s, Message: %s", status, reason, message)
}

// HaveStatus is a gomega matcher for a condition status, reason and message
func HaveStatus(status metav1.ConditionStatus, reason string, message string) gomegatypes.GomegaMatcher {
	return Equal(statusString(status, reason, message))
}

// ExpectCondition take a condition type and returns its status, reason and message
func ExpectCondition(
	pr *deploymentskyscannernetv1alpha1.ProgressiveRollout, ct string,
) AsyncAssertion {
	return Eventually(func() string {
		_ = k8sClient.Get(
			context.Background(),
			types.NamespacedName{Name: pr.Name, Namespace: pr.Namespace},
			pr,
		)
		for _, c := range pr.Status.Conditions {
			if c.Type == ct {
				return statusString(c.Status, c.Reason, c.Message)
			}
		}
		return ""
	})
}

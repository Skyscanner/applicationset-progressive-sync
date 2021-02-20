package controllers

import (
	"context"
	"fmt"
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"time"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 10
)

var _ = Describe("ProgressiveRollout Controller", func() {

	var (
		ctx                                 context.Context
		singleStagePR                       *deploymentskyscannernetv1alpha1.ProgressiveRollout
		namespace, argoApiGroup, appSetKind string
		ns                                  *corev1.Namespace
	)
	argoApiGroup = "argoproj.io/v1alpha1"
	appSetKind = "ApplicationSet"
	// See https://onsi.github.io/gomega#modifying-default-intervals
	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(interval)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "progressiverollout-test" + randStringNumber(5)

		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		singleStagePR = &deploymentskyscannernetv1alpha1.ProgressiveRollout{
			ObjectMeta: metav1.ObjectMeta{Name: "single-stage-pr", Namespace: namespace},
			Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
				SourceRef: corev1.TypedLocalObjectReference{
					APIGroup: &argoApiGroup,
					Kind:     "",
					Name:     "",
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
	})

	JustBeforeEach(func() {
		err := k8sClient.Create(context.Background(), ns)
		Expect(err).To(BeNil(), "failed to create test namespace")
	})

	AfterEach(func() {
		err := k8sClient.Delete(context.Background(), ns)
		Expect(err).To(BeNil(), "failed to delete test namespace")
	})

	Describe("requestsForApplicationChange function", func() {
		It("should forward events for owned applications", func() {
			By("creating a progressive rollout object with sourceRef")
			singleStagePR.Spec.SourceRef.Name = "single-stage-appset"
			singleStagePR.Spec.SourceRef.Kind = appSetKind
			Expect(k8sClient.Create(ctx, singleStagePR)).To(Succeed())

			By("creating an owned application")
			ownedApp := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: argoApiGroup,
						Kind:       appSetKind,
						Name:       "single-stage-appset",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			}
			Expect(k8sClient.Create(ctx, ownedApp)).To(Succeed())

			requests := reconciler.requestsForApplicationChange(ownedApp)
			Expect(len(requests)).To(Equal(1))
			Expect(requests[0].NamespacedName).To(Equal(types.NamespacedName{
				Namespace: namespace,
				Name:      "single-stage-pr",
			}))
		})

		It("should filter out events for non-owned applications", func() {

		})
	})

	Describe("Reconciliation loop", func() {
		It("should reconcile", func() {
			By("creating a progressive rollout object")
			Expect(k8sClient.Create(ctx, singleStagePR)).To(Succeed())

			expected := singleStagePR.NewStatusCondition(deploymentskyscannernetv1alpha1.CompletedCondition, metav1.ConditionTrue, deploymentskyscannernetv1alpha1.StagesCompleteReason, "All stages completed")
			ExpectCondition(singleStagePR, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))
		})
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

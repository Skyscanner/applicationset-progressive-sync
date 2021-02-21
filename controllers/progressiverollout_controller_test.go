package controllers

import (
	"context"
	"fmt"
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	"github.com/Skyscanner/argocd-progressive-rollout/internal"
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
		ctx                     context.Context
		namespace, appSetAPIRef string
		ns                      *corev1.Namespace
		ownerPR, singleStagePR  *deploymentskyscannernetv1alpha1.ProgressiveRollout
	)

	appSetAPIRef = internal.AppSetAPIGroup
	// See https://onsi.github.io/gomega#modifying-default-intervals
	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(interval)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "progressiverollout-test" + randStringNumber(5)

		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		err := k8sClient.Create(context.Background(), ns)
		Expect(err).To(BeNil(), "failed to create test namespace")

	})

	AfterEach(func() {
		err := k8sClient.Delete(context.Background(), ns)
		Expect(err).To(BeNil(), "failed to delete test namespace")
	})

	Describe("requestsForApplicationChange function", func() {

		BeforeEach(func() {
			ownerPR = &deploymentskyscannernetv1alpha1.ProgressiveRollout{
				ObjectMeta: metav1.ObjectMeta{Name: "owner-pr", Namespace: namespace},
				Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     internal.AppSetKind,
						Name:     "owner-app-set",
					},
					Stages: nil,
				}}
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})

		It("should forward events for owned applications", func() {
			By("creating an owned application")
			ownedApp := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: internal.AppSetAPIGroup,
						Kind:       internal.AppSetKind,
						Name:       "owner-app-set",
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
				Name:      "owner-pr",
			}))
		})

		It("should filter out events for non-owned applications", func() {
			By("creating a non-owned application")
			nonOwnedApp := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-owned-app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: internal.AppSetAPIGroup,
						Kind:       internal.AppSetKind,
						Name:       "not-owned",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			}
			Expect(k8sClient.Create(ctx, nonOwnedApp)).To(Succeed())

			requests := reconciler.requestsForApplicationChange(nonOwnedApp)
			Expect(len(requests)).To(Equal(0))
		})
	})

	Describe("requestsForSecretChange function", func() {

		BeforeEach(func() {
			ownerPR = &deploymentskyscannernetv1alpha1.ProgressiveRollout{
				ObjectMeta: metav1.ObjectMeta{Name: "owner-pr", Namespace: namespace},
				Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     internal.AppSetKind,
						Name:     "owner-app-set",
					},
					Stages: nil,
				}}
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})

		It("should forward an event for a matching argocd secret", func() {
			serverURL := "https://kubernetes.default.svc"
			By("creating an application")
			app := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: internal.AppSetAPIGroup,
						Kind:       internal.AppSetKind,
						Name:       "owner-app-set",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
					Server:    serverURL,
					Namespace: namespace,
					Name:      "local-cluster",
				}},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("creating a cluster secret")
			cluster := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace, Labels: map[string]string{internal.ArgoCDSecretTypeLabel: internal.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte(serverURL)}}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			requests := reconciler.requestsForSecretChange(cluster)
			Expect(len(requests)).To(Equal(1))
		})

		It("should not forward an event for a generic secret", func() {
			By("creating a generic secret")
			generic := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "generic", Namespace: namespace}, Data: map[string][]byte{"secret": []byte("insecure")},
			}
			Expect(k8sClient.Create(ctx, generic)).To(Succeed())

			requests := reconciler.requestsForSecretChange(generic)
			Expect(len(requests)).To(Equal(0))
		})

		It("should not forward an event for an argocd secret not matching any application", func() {
			By("creating an application")
			app := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: internal.AppSetAPIGroup,
						Kind:       internal.AppSetKind,
						Name:       "owner-app-set",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://remote-url.kubernetes.io",
					Namespace: namespace,
					Name:      "remote-cluster",
				}},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("creating a cluster secret")
			cluster := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace, Labels: map[string]string{internal.ArgoCDSecretTypeLabel: internal.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte("https://kubernetes.default.svc")}}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			requests := reconciler.requestsForSecretChange(cluster)
			Expect(len(requests)).To(Equal(0))
		})
	})

	Describe("Reconciliation loop", func() {
		It("should reconcile", func() {
			By("creating a progressive rollout object")
			singleStagePR = &deploymentskyscannernetv1alpha1.ProgressiveRollout{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-pr", Namespace: namespace},
				Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
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

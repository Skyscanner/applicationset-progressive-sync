package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	"github.com/Skyscanner/argocd-progressive-rollout/internal/utils"
	"github.com/Skyscanner/argocd-progressive-rollout/mocks"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

	appSetAPIRef = utils.AppSetAPIGroup
	// See https://onsi.github.io/gomega#modifying-default-intervals
	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(interval)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "progressiverollout-test-" + randStringNumber(5)

		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		err := k8sClient.Create(context.Background(), ns)
		Expect(err).To(BeNil(), "failed to create test namespace")

		reconciler.ArgoCDAppClient = &mocks.ArgoCDAppClientStub{}
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
						Kind:     utils.AppSetKind,
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
						APIVersion: utils.AppSetAPIGroup,
						Kind:       utils.AppSetKind,
						Name:       "owner-app-set",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			}

			var requests []reconcile.Request
			Eventually(func() int {
				requests = reconciler.requestsForApplicationChange(ownedApp)
				return len(requests)
			}).Should(Equal(1))
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
						APIVersion: utils.AppSetAPIGroup,
						Kind:       utils.AppSetKind,
						Name:       "not-owned",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			}

			Eventually(func() int {
				requests := reconciler.requestsForApplicationChange(nonOwnedApp)
				return len(requests)
			}).Should(Equal(0))
		})
	})

	Describe("requestsForSecretChange function", func() {

		BeforeEach(func() {
			ownerPR = &deploymentskyscannernetv1alpha1.ProgressiveRollout{
				ObjectMeta: metav1.ObjectMeta{Name: "owner-pr", Namespace: namespace},
				Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     utils.AppSetKind,
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
						APIVersion: utils.AppSetAPIGroup,
						Kind:       utils.AppSetKind,
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
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace, Labels: map[string]string{utils.ArgoCDSecretTypeLabel: utils.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte(serverURL)}}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(cluster)
				return len(requests)
			}).Should(Equal(1))
		})

		It("should not forward an event for a generic secret", func() {
			By("creating a generic secret")
			generic := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "generic", Namespace: namespace}, Data: map[string][]byte{"secret": []byte("insecure")},
			}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(generic)
				return len(requests)
			}).Should(Equal(0))
		})

		It("should not forward an event for an argocd secret not matching any application", func() {
			By("creating an application")
			externalApp := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: utils.AppSetAPIGroup,
						Kind:       utils.AppSetKind,
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
			Expect(k8sClient.Create(ctx, externalApp)).To(Succeed())

			By("creating a cluster secret")
			internalCluster := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace, Labels: map[string]string{utils.ArgoCDSecretTypeLabel: utils.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte("https://local-kubernetes.default.svc")}}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(internalCluster)
				return len(requests)
			}).Should(Equal(0))
		})
	})

	Describe("removeAnnotationFromApps function", func() {
		It("should remove the target annotation from the given apps", func() {
			By("creating two applications with two annotations")
			appOne := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-one",
					Namespace: namespace,
					Annotations: map[string]string{
						"foo": "bar",
						"key": "value",
					},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			}
			Expect(k8sClient.Create(ctx, &appOne)).To(Succeed())

			appTwo := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-two",
					Namespace: namespace,
					Annotations: map[string]string{
						"bob": "alice",
						"key": "value",
					},
				},
				Spec: argov1alpha1.ApplicationSpec{},
			}
			Expect(k8sClient.Create(ctx, &appTwo)).To(Succeed())

			apps := []argov1alpha1.Application{appOne, appTwo}
			err := reconciler.removeAnnotationFromApps(apps, "key")
			Expect(err).To(BeNil())
			Eventually(func() int { return len(appOne.Annotations) }).Should(Equal(1))
			Eventually(func() int { return len(appTwo.Annotations) }).Should(Equal(1))
		})
	})

	Describe("Reconciliation loop", func() {
		It("should reconcile", func() {
			By("creating an ArgoCD cluster")
			cluster := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-cluster", Namespace: namespace, Labels: map[string]string{utils.ArgoCDSecretTypeLabel: utils.ArgoCDSecretTypeCluster}},
				Data: map[string][]byte{
					"server": []byte("https://single-stage-pr.kubernetes.io"),
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("creating an application targeting the cluster")
			singleStageApp := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "single-stage-app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: utils.AppSetAPIGroup,
						Kind:       utils.AppSetKind,
						Name:       "single-stage-appset",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://single-stage-pr.kubernetes.io",
					Namespace: namespace,
					Name:      "remote-cluster",
				}},
				Status: argov1alpha1.ApplicationStatus{Sync: argov1alpha1.SyncStatus{Status: argov1alpha1.SyncStatusCodeOutOfSync}},
			}
			Expect(k8sClient.Create(ctx, singleStageApp)).To(Succeed())

			By("creating a progressive rollout")
			singleStagePR = &deploymentskyscannernetv1alpha1.ProgressiveRollout{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-pr", Namespace: namespace},
				Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     utils.AppSetKind,
						Name:     "single-stage-appset",
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

			createdPR := deploymentskyscannernetv1alpha1.ProgressiveRollout{}
			Eventually(func() int {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(singleStagePR), &createdPR)
				return len(createdPR.ObjectMeta.Finalizers)
			}).Should(Equal(1))
			Expect(createdPR.ObjectMeta.Finalizers[0]).To(Equal(deploymentskyscannernetv1alpha1.ProgressiveRolloutFinalizer))

			expected := singleStagePR.NewStatusCondition(deploymentskyscannernetv1alpha1.CompletedCondition, metav1.ConditionTrue, deploymentskyscannernetv1alpha1.StagesCompleteReason, "All stages completed")
			ExpectCondition(singleStagePR, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))

			deletedPR := deploymentskyscannernetv1alpha1.ProgressiveRollout{}
			Expect(k8sClient.Delete(ctx, singleStagePR)).To(Succeed())
			Eventually(func() error {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(singleStagePR), &deletedPR)
				return err
			}).Should(HaveOccurred())
		})

		It("should send a request to sync an application", func() {
			mockedArgoCDAppClient := &mocks.MockArgoCDAppClientCalledWith{}
			reconciler.ArgoCDAppClient = mockedArgoCDAppClient
			testAppName := "single-stage-app"

			By("creating an ArgoCD cluster")
			cluster := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-cluster", Namespace: namespace, Labels: map[string]string{utils.ArgoCDSecretTypeLabel: utils.ArgoCDSecretTypeCluster}},
				Data: map[string][]byte{
					"server": []byte("https://single-stage-pr.kubernetes.io"),
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("creating an application targeting the cluster")
			singleStageApp := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: utils.AppSetAPIGroup,
						Kind:       utils.AppSetKind,
						Name:       "single-stage-appset",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://single-stage-pr.kubernetes.io",
					Namespace: namespace,
					Name:      "remote-cluster",
				}},
				Status: argov1alpha1.ApplicationStatus{Sync: argov1alpha1.SyncStatus{Status: argov1alpha1.SyncStatusCodeOutOfSync}},
			}
			Expect(k8sClient.Create(ctx, singleStageApp)).To(Succeed())

			By("creating a progressive rollout")
			singleStagePR = &deploymentskyscannernetv1alpha1.ProgressiveRollout{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-pr", Namespace: namespace},
				Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     utils.AppSetKind,
						Name:     "single-stage-appset",
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

			Eventually(func() []string {
				return mockedArgoCDAppClient.GetSyncedApps()
			}).Should(ContainElement(testAppName))
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

func TestSync(t *testing.T) {
	r := ProgressiveRolloutReconciler{
		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncOK{},
	}

	testAppName := "foo-bar"

	application, error := r.syncApp(testAppName)

	g := NewWithT(t)
	g.Expect(error).To(BeNil())
	g.Expect(application.Name).To(Equal(testAppName))
}

func TestSyncErr(t *testing.T) {
	r := ProgressiveRolloutReconciler{
		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncNotOK{},
	}

	testAppName := "foo-bar"

	application, error := r.syncApp(testAppName)

	g := NewWithT(t)
	g.Expect(application).To(BeNil())
	g.Expect(error).ToNot(BeNil())
}

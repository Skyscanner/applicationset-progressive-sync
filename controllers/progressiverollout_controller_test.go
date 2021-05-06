package controllers

import (
	"context"
	"fmt"
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	"github.com/Skyscanner/argocd-progressive-rollout/internal/utils"
	"github.com/Skyscanner/argocd-progressive-rollout/mocks"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
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

	Describe("Sync application", func() {
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

	Describe("Reconciliation loop", func() {
		It("should reconcile two stages", func() {
			testPrefix := "two-stages"

			By("creating 2 ArgoCD cluster")
			clusters, cErr := createClusters(ctx, namespace, testPrefix, 2)
			Expect(clusters).To(Not(BeNil()))
			Expect(cErr).To(BeNil())

			By("creating one application targeting each cluster")
			appOne, aErr := createApplication(ctx, namespace, testPrefix, clusters[0])
			Expect(aErr).To(BeNil())
			appTwo, aErr := createApplication(ctx, namespace, testPrefix, clusters[1])
			Expect(aErr).To(BeNil())

			By("creating a progressive rollout")
			twoStagesPR := &deploymentskyscannernetv1alpha1.ProgressiveRollout{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-pr", testPrefix), Namespace: namespace},
				Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     utils.AppSetKind,
						Name:     fmt.Sprintf("%s-appset", testPrefix),
					},
					Stages: []deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{{
						Name:        "stage 0",
						MaxParallel: intstr.IntOrString{IntVal: 1},
						MaxTargets:  intstr.IntOrString{IntVal: 1},
						Targets: deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{Clusters: deploymentskyscannernetv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster.name": clusters[0].Name,
							}},
						}},
					}, {
						Name:        "stage 1",
						MaxParallel: intstr.IntOrString{IntVal: 1},
						MaxTargets:  intstr.IntOrString{IntVal: 1},
						Targets: deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{Clusters: deploymentskyscannernetv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster.name": clusters[1].Name,
							}},
						}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, twoStagesPR)).To(Succeed())

			prKey := client.ObjectKey{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-pr", testPrefix),
			}

			By("progressing in first application")
			app := &argov1alpha1.Application{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: namespace,
					Name:      appOne.Name,
				}, app)
			}).Should(Succeed())
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusProgressing,
				Message: "progressing",
			}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())

			ExpectStageStatusPhase(ctx, prKey, "stage 0").Should(Equal(deploymentskyscannernetv1alpha1.PhaseProgressing))
			ExpectStagesInStatus(ctx, prKey).Should(Equal(1))

			By("finishing first application")
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "healthy",
			}
			app.Status.Sync = argov1alpha1.SyncStatus{
				Status: argov1alpha1.SyncStatusCodeSynced,
			}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())

			ExpectStageStatusPhase(ctx, prKey, "stage 0").Should(Equal(deploymentskyscannernetv1alpha1.PhaseSucceeded))

			By("progressing in second application")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: namespace,
					Name:      appTwo.Name,
				}, app)
			}).Should(Succeed())
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusProgressing,
				Message: "progressing",
			}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())

			ExpectStageStatusPhase(ctx, prKey, "stage 1").Should(Equal(deploymentskyscannernetv1alpha1.PhaseProgressing))
			ExpectStagesInStatus(ctx, prKey).Should(Equal(2))

			By("finishing second application")
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "healthy",
			}
			app.Status.Sync = argov1alpha1.SyncStatus{
				Status: argov1alpha1.SyncStatusCodeSynced,
			}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())

			ExpectStageStatusPhase(ctx, prKey, "stage 1").Should(Equal(deploymentskyscannernetv1alpha1.PhaseSucceeded))

			expected := twoStagesPR.NewStatusCondition(deploymentskyscannernetv1alpha1.CompletedCondition, metav1.ConditionTrue, deploymentskyscannernetv1alpha1.StagesCompleteReason, "All stages completed")
			ExpectCondition(twoStagesPR, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))
		})
	})
})

func ExpectStageStatusPhase(ctx context.Context, prKey client.ObjectKey, stageName string) AsyncAssertion {
	return Eventually(func() interface{} {
		pr := &deploymentskyscannernetv1alpha1.ProgressiveRollout{}
		err := k8sClient.Get(ctx, prKey, pr)
		if err != nil {
			return nil
		}
		stageStatus := deploymentskyscannernetv1alpha1.FindStageStatus(pr.Status.Stages, stageName)
		if stageStatus != nil {
			return stageStatus.Phase
		}

		return nil
	})
}

func ExpectStagesInStatus(ctx context.Context, prKey client.ObjectKey) AsyncAssertion {
	return Eventually(func() int {
		pr := &deploymentskyscannernetv1alpha1.ProgressiveRollout{}
		err := k8sClient.Get(ctx, prKey, pr)
		if err != nil {
			return -1
		}

		return len(pr.Status.Stages)
	})
}

func createClusters(ctx context.Context, namespace string, prefix string, number int) ([]corev1.Secret, error) {
	var clusters []corev1.Secret

	for i := 0; i < number; i++ {
		clusterName := fmt.Sprintf("%s-cluster-%d", prefix, i)
		cluster := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: namespace, Labels: map[string]string{
				utils.ArgoCDSecretTypeLabel: utils.ArgoCDSecretTypeCluster,
				"cluster.name":              clusterName,
			}},
			Data: map[string][]byte{
				"server": []byte(fmt.Sprintf("https://%s.kubernetes.io", clusterName)),
			},
		}

		err := k8sClient.Create(ctx, &cluster)
		if err != nil {
			return nil, err
		}

		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

func createApplication(ctx context.Context, namespace string, prefix string, cluster corev1.Secret) (*argov1alpha1.Application, error) {
	appSetName := fmt.Sprintf("%s-appset", prefix)

	appName := fmt.Sprintf("%s-app-%s", prefix, cluster.Name)
	app := &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: utils.AppSetAPIGroup,
				Kind:       utils.AppSetKind,
				Name:       appSetName,
				UID:        uuid.NewUUID(),
			}},
		},
		Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
			Server:    string(cluster.Data["server"]),
			Namespace: namespace,
			Name:      cluster.Name,
		}},
		Status: argov1alpha1.ApplicationStatus{
			Sync: argov1alpha1.SyncStatus{
				Status: argov1alpha1.SyncStatusCodeOutOfSync,
			},
			Health: argov1alpha1.HealthStatus{
				Status: health.HealthStatusHealthy,
			},
		},
	}

	err := k8sClient.Create(ctx, app)
	if err != nil {
		return nil, err
	}

	return app, nil
}

// statusString returns a formatted string with a condition status, reason and message
func statusString(status metav1.ConditionStatus, reason string, message string) string {
	return fmt.Sprintf("Status: %s, Reason: %s, Message: %s", status, reason, message)
}

// HaveStatus is a gomega matcher for a condition status, reason and message
func HaveStatus(status metav1.ConditionStatus, reason string, message string) gomegatypes.GomegaMatcher {
	return Equal(statusString(status, reason, message))
}

// MatchStage is a gomega matcher for a stage, matching name, message and phase
func MatchStage(expected deploymentskyscannernetv1alpha1.StageStatus) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Name":    Equal(expected.Name),
		"Message": Equal(expected.Message),
		"Phase":   Equal(expected.Phase),
	})
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

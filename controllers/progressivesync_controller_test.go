package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/utils"
	"github.com/Skyscanner/applicationset-progressive-sync/mocks"
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
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 10
)

var (
	appSetAPIRef = utils.AppSetAPIGroup
	ctx          = context.Background()
)

func createRandomNamespace() (string, *corev1.Namespace) {
	namespace := "progressiverollout-test-" + randStringNumber(5)

	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}
	Expect(k8sClient.Create(ctx, &ns)).To(Succeed())
	return namespace, &ns
}

func createOwnerPR(ns string, owner string) *syncv1alpha1.ProgressiveSync {
	return &syncv1alpha1.ProgressiveSync{
		ObjectMeta: metav1.ObjectMeta{Name: owner, Namespace: ns},
		Spec: syncv1alpha1.ProgressiveSyncSpec{
			SourceRef: corev1.TypedLocalObjectReference{
				APIGroup: &appSetAPIRef,
				Kind:     utils.AppSetKind,
				Name:     "owner-app-set",
			},
			Stages: nil,
		}}
}

var _ = Describe("ProgressiveRollout Controller", func() {

	appSetAPIRef = utils.AppSetAPIGroup
	// See https://onsi.github.io/gomega#modifying-default-intervals
	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(interval)

	var (
		namespace string
		ns        *corev1.Namespace
	)

	BeforeEach(func() {
		namespace, ns = createRandomNamespace()
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})

	Describe("requestsForApplicationChange function", func() {

		It("should forward events for owned applications", func() {
			By("creating an owned application")

			ownerPR := createOwnerPR(namespace, "owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			ownedApp := argov1alpha1.Application{
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
				requests = reconciler.requestsForApplicationChange(&ownedApp)
				return len(requests)
			}).Should(Equal(1))
			Expect(requests[0].NamespacedName).To(Equal(types.NamespacedName{
				Namespace: namespace,
				Name:      "owner-pr",
			}))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})

		It("should filter out events for non-owned applications", func() {
			By("creating a non-owned application")
			ownerPR := createOwnerPR(namespace, "owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			nonOwnedApp := argov1alpha1.Application{
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
				requests := reconciler.requestsForApplicationChange(&nonOwnedApp)
				return len(requests)
			}).Should(Equal(0))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})
	})

	Describe("requestsForSecretChange function", func() {

		It("should forward an event for a matching argocd secret", func() {
			ownerPR := createOwnerPR(namespace, "owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			serverURL := "https://kubernetes.default.svc"

			By("creating an application")
			app := argov1alpha1.Application{
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
			Expect(k8sClient.Create(ctx, &app)).To(Succeed())

			By("creating a cluster secret")
			cluster := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace, Labels: map[string]string{utils.ArgoCDSecretTypeLabel: utils.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte(serverURL)}}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(&cluster)
				return len(requests)
			}).Should(Equal(1))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})

		It("should not forward an event for a generic secret", func() {
			ownerPR := createOwnerPR(namespace, "owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			By("creating a generic secret")
			generic := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "generic", Namespace: namespace}, Data: map[string][]byte{"secret": []byte("insecure")},
			}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(&generic)
				return len(requests)
			}).Should(Equal(0))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})

		It("should not forward an event for an argocd secret not matching any application", func() {
			ownerPR := createOwnerPR(namespace, "owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())

			By("creating an application")
			externalApp := argov1alpha1.Application{
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
			Expect(k8sClient.Create(ctx, &externalApp)).To(Succeed())

			By("creating a cluster secret")
			internalCluster := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace, Labels: map[string]string{utils.ArgoCDSecretTypeLabel: utils.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte("https://local-kubernetes.default.svc")}}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(&internalCluster)
				return len(requests)
			}).Should(Equal(0))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
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
			err := reconciler.removeAnnotationFromApps(ctx, apps, "key")
			Expect(err).To(BeNil())
			Eventually(func() int { return len(appOne.Annotations) }).Should(Equal(1))
			Eventually(func() int { return len(appTwo.Annotations) }).Should(Equal(1))
		})
	})

	Describe("reconciliation loop", func() {
		It("should reconcile two stages", func() {
			testPrefix := "two-stages"

			By("creating 2 ArgoCD cluster")
			clusters, cErr := createClusters(ctx, namespace, testPrefix, 2)
			Expect(clusters).To(Not(BeNil()))
			Expect(cErr).To(BeNil())

			By("creating one application targeting each cluster")
			appOne, aErr := createApplication(ctx, testPrefix, clusters[0])
			Expect(aErr).To(BeNil())
			appTwo, aErr := createApplication(ctx, testPrefix, clusters[1])
			Expect(aErr).To(BeNil())

			By("creating a progressive sync")
			twoStagesPR := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-pr", testPrefix), Namespace: namespace},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     utils.AppSetKind,
						Name:     fmt.Sprintf("%s-appset", testPrefix),
					},
					Stages: []syncv1alpha1.ProgressiveSyncStage{{
						Name:        "stage 0",
						MaxParallel: intstr.IntOrString{IntVal: 1},
						MaxTargets:  intstr.IntOrString{IntVal: 1},
						Targets: syncv1alpha1.ProgressiveSyncTargets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster.name": clusters[0].Name,
							}},
						}},
					}, {
						Name:        "stage 1",
						MaxParallel: intstr.IntOrString{IntVal: 1},
						MaxTargets:  intstr.IntOrString{IntVal: 1},
						Targets: syncv1alpha1.ProgressiveSyncTargets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster.name": clusters[1].Name,
							}},
						}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &twoStagesPR)).To(Succeed())

			prKey := client.ObjectKey{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-pr", testPrefix),
			}

			By("progressing in first application")
			app := argov1alpha1.Application{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: namespace,
					Name:      appOne.Name,
				}, &app)
			}).Should(Succeed())
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusProgressing,
				Message: "progressing",
			}
			Expect(k8sClient.Update(ctx, &app)).To(Succeed())

			ExpectStageStatus(ctx, prKey, "stage 0").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "stage 0",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: "stage in progress",
			}))
			ExpectStagesInStatus(ctx, prKey).Should(Equal(1))

			By("finishing first application")
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "healthy",
			}
			app.Status.Sync = argov1alpha1.SyncStatus{
				Status: argov1alpha1.SyncStatusCodeSynced,
			}
			Expect(k8sClient.Update(ctx, &app)).To(Succeed())

			ExpectStageStatus(ctx, prKey, "stage 0").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "stage 0",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "stage completed",
			}))

			By("progressing in second application")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: namespace,
					Name:      appTwo.Name,
				}, &app)
			}).Should(Succeed())
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusProgressing,
				Message: "progressing",
			}
			Expect(k8sClient.Update(ctx, &app)).To(Succeed())

			ExpectStageStatus(ctx, prKey, "stage 1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "stage 1",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: "stage in progress",
			}))
			ExpectStagesInStatus(ctx, prKey).Should(Equal(2))

			By("finishing second application")
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "healthy",
			}
			app.Status.Sync = argov1alpha1.SyncStatus{
				Status: argov1alpha1.SyncStatusCodeSynced,
			}
			Expect(k8sClient.Update(ctx, &app)).To(Succeed())

			ExpectStageStatus(ctx, prKey, "stage 1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "stage 1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "stage completed",
			}))

			createdPR := syncv1alpha1.ProgressiveSync{}
			Eventually(func() int {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&twoStagesPR), &createdPR)
				return len(createdPR.ObjectMeta.Finalizers)
			}).Should(Equal(1))
			Expect(createdPR.ObjectMeta.Finalizers[0]).To(Equal(syncv1alpha1.ProgressiveSyncFinalizer))

			expected := twoStagesPR.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesCompleteReason, "All stages completed")
			ExpectCondition(&twoStagesPR, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))

			deletedPR := syncv1alpha1.ProgressiveSync{}
			Expect(k8sClient.Delete(ctx, &twoStagesPR)).To(Succeed())
			Eventually(func() error {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&twoStagesPR), &deletedPR)
				return err
			}).Should(HaveOccurred())
		})

		It("should fail if unable to sync application", func() {
			testPrefix := "failed-stages"

			By("creating 2 ArgoCD cluster")
			clusters, cErr := createClusters(ctx, namespace, testPrefix, 2)
			Expect(clusters).To(Not(BeNil()))
			Expect(cErr).To(BeNil())

			By("creating one application targeting each cluster")
			appOne, aErr := createApplication(ctx, testPrefix, clusters[0])
			Expect(aErr).To(BeNil())
			_, aErr = createApplication(ctx, testPrefix, clusters[1])
			Expect(aErr).To(BeNil())

			By("creating a progressive sync")
			failedStagePR := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-pr", testPrefix), Namespace: namespace},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     utils.AppSetKind,
						Name:     fmt.Sprintf("%s-appset", testPrefix),
					},
					Stages: []syncv1alpha1.ProgressiveSyncStage{{
						Name:        "stage 0",
						MaxParallel: intstr.IntOrString{IntVal: 1},
						MaxTargets:  intstr.IntOrString{IntVal: 1},
						Targets: syncv1alpha1.ProgressiveSyncTargets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster.name": clusters[0].Name,
							}},
						}},
					}, {
						Name:        "stage 1",
						MaxParallel: intstr.IntOrString{IntVal: 1},
						MaxTargets:  intstr.IntOrString{IntVal: 1},
						Targets: syncv1alpha1.ProgressiveSyncTargets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster.name": clusters[1].Name,
							}},
						}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &failedStagePR)).To(Succeed())

			prKey := client.ObjectKey{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-pr", testPrefix),
			}

			By("progressing in first application")
			app := argov1alpha1.Application{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: namespace,
					Name:      appOne.Name,
				}, &app)
			}).Should(Succeed())
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusProgressing,
				Message: "progressing",
			}
			Expect(k8sClient.Update(ctx, &app)).To(Succeed())

			ExpectStageStatus(ctx, prKey, "stage 0").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "stage 0",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: "stage in progress",
			}))
			ExpectStagesInStatus(ctx, prKey).Should(Equal(1))

			By("failed syncing first application")
			app.Status.Health = argov1alpha1.HealthStatus{
				Status:  health.HealthStatusDegraded,
				Message: "healthy",
			}
			app.Status.Sync = argov1alpha1.SyncStatus{
				Status: argov1alpha1.SyncStatusCodeSynced,
			}
			Expect(k8sClient.Update(ctx, &app)).To(Succeed())

			ExpectStageStatus(ctx, prKey, "stage 0").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "stage 0",
				Phase:   syncv1alpha1.PhaseFailed,
				Message: "stage failed",
			}))

			createdPR := syncv1alpha1.ProgressiveSync{}
			Eventually(func() int {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&failedStagePR), &createdPR)
				return len(createdPR.ObjectMeta.Finalizers)
			}).Should(Equal(1))
			Expect(createdPR.ObjectMeta.Finalizers[0]).To(Equal(syncv1alpha1.ProgressiveSyncFinalizer))

			expected := failedStagePR.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesFailedReason, "stage 0 stage failed")
			ExpectCondition(&failedStagePR, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))

			deletedPR := syncv1alpha1.ProgressiveSync{}
			Expect(k8sClient.Delete(ctx, &failedStagePR)).To(Succeed())
			Eventually(func() error {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&failedStagePR), &deletedPR)
				return err
			}).Should(HaveOccurred())
		})
	})

	Describe("sync application", func() {
		It("should send a request to sync an application", func() {
			mockedArgoCDAppClient := mocks.MockArgoCDAppClientCalledWith{}
			reconciler.ArgoCDAppClient = &mockedArgoCDAppClient
			testAppName := "single-stage-app"

			By("creating an ArgoCD cluster")
			cluster := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-cluster", Namespace: namespace, Labels: map[string]string{utils.ArgoCDSecretTypeLabel: utils.ArgoCDSecretTypeCluster}},
				Data: map[string][]byte{
					"server": []byte("https://single-stage-pr.kubernetes.io"),
				},
			}
			Expect(k8sClient.Create(ctx, &cluster)).To(Succeed())

			By("creating an application targeting the cluster")
			singleStageApp := argov1alpha1.Application{
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
			Expect(k8sClient.Create(ctx, &singleStageApp)).To(Succeed())

			By("creating a progressive sync")
			singleStagePR := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-pr", Namespace: namespace},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     utils.AppSetKind,
						Name:     "single-stage-appset",
					},
					Stages: []syncv1alpha1.ProgressiveSyncStage{{
						Name:        "stage 1",
						MaxParallel: intstr.IntOrString{IntVal: 1},
						MaxTargets:  intstr.IntOrString{IntVal: 1},
						Targets: syncv1alpha1.ProgressiveSyncTargets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: nil},
						}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &singleStagePR)).To(Succeed())

			Eventually(func() []string {
				return mockedArgoCDAppClient.GetSyncedApps()
			}).Should(ContainElement(testAppName))
		})
	})
})

// createClusters is a helper function that creates N clusters in a given namespace with a common name prefix
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

// createApplication is a helper function that creates an ArgoCD application given a prefix and a cluster
func createApplication(ctx context.Context, prefix string, cluster corev1.Secret) (*argov1alpha1.Application, error) {
	appSetName := fmt.Sprintf("%s-appset", prefix)

	appName := fmt.Sprintf("%s-app-%s", prefix, cluster.Name)
	app := argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: utils.AppSetAPIGroup,
				Kind:       utils.AppSetKind,
				Name:       appSetName,
				UID:        uuid.NewUUID(),
			}},
		},
		Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
			Server:    string(cluster.Data["server"]),
			Namespace: cluster.Namespace,
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

	err := k8sClient.Create(ctx, &app)
	if err != nil {
		return nil, err
	}

	return &app, nil
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
func MatchStage(expected syncv1alpha1.StageStatus) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Name":    Equal(expected.Name),
		"Message": Equal(expected.Message),
		"Phase":   Equal(expected.Phase),
	})
}

// ExpectCondition take a condition type and returns its status, reason and message
func ExpectCondition(
	pr *syncv1alpha1.ProgressiveSync, ct string,
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

// ExpectStageStatus returns an AsyncAssertion for a StageStatus, given a progressive sync Object Key and a stage name
func ExpectStageStatus(ctx context.Context, prKey client.ObjectKey, stageName string) AsyncAssertion {
	return Eventually(func() syncv1alpha1.StageStatus {
		pr := syncv1alpha1.ProgressiveSync{}
		err := k8sClient.Get(ctx, prKey, &pr)
		if err != nil {
			return syncv1alpha1.StageStatus{}
		}
		stageStatus := syncv1alpha1.FindStageStatus(pr.Status.Stages, stageName)
		if stageStatus != nil {
			return *stageStatus
		}

		return syncv1alpha1.StageStatus{}
	})
}

// ExpectStagesInStatus returns an AsyncAssertion for the length of stages with status in the Progressive Rollout object
func ExpectStagesInStatus(ctx context.Context, prKey client.ObjectKey) AsyncAssertion {
	return Eventually(func() int {
		pr := syncv1alpha1.ProgressiveSync{}
		err := k8sClient.Get(ctx, prKey, &pr)
		if err != nil {
			return -1
		}

		return len(pr.Status.Stages)
	})
}

func TestSync(t *testing.T) {
	r := ProgressiveSyncReconciler{
		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncOK{},
	}

	testAppName := "foo-bar"

	application, error := r.syncApp(ctx, testAppName)

	g := NewWithT(t)
	g.Expect(error).To(BeNil())
	g.Expect(application.Name).To(Equal(testAppName))
}

func TestSyncErr(t *testing.T) {
	r := ProgressiveSyncReconciler{
		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncNotOK{},
	}

	testAppName := "foo-bar"

	application, error := r.syncApp(ctx, testAppName)

	g := NewWithT(t)
	g.Expect(application).To(BeNil())
	g.Expect(error).ToNot(BeNil())
}

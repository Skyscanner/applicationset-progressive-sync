package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/utils"
	"github.com/Skyscanner/applicationset-progressive-sync/mocks"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	timeout  = time.Second * 360
	interval = time.Millisecond * 10
)

var (
	appSetAPIRef = consts.AppSetAPIGroup
	ctx          context.Context
	cancel       context.CancelFunc
)

// Target is an helper structure that holds the information to create clusters and applications
type Target struct {
	Name           string
	Namespace      string
	ApplicationSet string
	Area           string
	Region         string
	AZ             string
}

func createRandomNamespace() (string, *corev1.Namespace) {
	namespace := "progressivesync-test-" + randStringNumber(5)

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
				Kind:     consts.AppSetKind,
				Name:     "owner-app-set",
			},
			Stages: nil,
		}}
}

var _ = Describe("ProgressiveRollout Controller", func() {

	appSetAPIRef = consts.AppSetAPIGroup
	// See https://onsi.github.io/gomega#modifying-default-intervals
	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(interval)
	format.TruncatedDiff = false

	var (
		namespace string
		ns        *corev1.Namespace
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		namespace, ns = createRandomNamespace()

		k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:    scheme.Scheme,
			Namespace: namespace,
		})
		Expect(err).ToNot(HaveOccurred())

		mockAcdClient := mocks.ArgoCDAppClientStub{}
		reconciler = &ProgressiveSyncReconciler{
			Client:          k8sManager.GetClient(),
			Log:             ctrl.Log.WithName("controllers").WithName("progressivesync"),
			ArgoCDAppClient: &mockAcdClient,
			StateManager:    utils.NewProgressiveSyncManager(),
		}
		err = reconciler.SetupWithManager(k8sManager)
		Expect(err).ToNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			err = k8sManager.Start(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
	})

	AfterEach(func() {
		defer cancel()
		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})

	Describe("requestsForApplicationChange function", func() {

		It("should forward events for owned applications", func() {
			By("creating an owned application")
			localNS := namespace
			ownerPR := createOwnerPR(localNS, "owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			ownedApp := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "owned-app",
					Namespace: localNS,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: consts.AppSetAPIGroup,
						Kind:       consts.AppSetKind,
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
				Namespace: localNS,
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
						APIVersion: consts.AppSetAPIGroup,
						Kind:       consts.AppSetKind,
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
			ownerPR := createOwnerPR(namespace, "secret-owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			serverURL := "https://kubernetes.default.svc"

			By("creating an application")
			app := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "matching-app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: consts.AppSetAPIGroup,
						Kind:       consts.AppSetKind,
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
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace, Labels: map[string]string{consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte(serverURL)}}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(&cluster)
				return len(requests)
			}).Should(Equal(1))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})

		It("should not forward an event for a generic secret", func() {
			ownerPR := createOwnerPR(namespace, "generic-owner-pr")
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
					Name:      "non-matching-app",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: consts.AppSetAPIGroup,
						Kind:       consts.AppSetKind,
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
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: namespace, Labels: map[string]string{consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte("https://local-kubernetes.default.svc")}}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(&internalCluster)
				return len(requests)
			}).Should(Equal(0))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})
	})

	Describe("reconciliation loop", func() {
		It("should reconcile a multi-stage progressive sync", func() {
			testPrefix := "multi"
			appSet := fmt.Sprintf("%s-appset", testPrefix)

			By("creating eight argocd clusters")
			targets := []Target{
				{
					Name:           "account1-eu-west-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1a",
				}, {
					Name:           "account1-eu-west-1a-2",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1a",
				}, {
					Name:           "account2-eu-central-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-central-1",
					AZ:             "eu-central-1a",
				}, {
					Name:           "account2-eu-central-1b-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-central-1",
					AZ:             "eu-central-1b",
				}, {
					Name:           "account3-ap-southeast-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "apac",
					Region:         "ap-southeast-1",
					AZ:             "ap-southeast-1a",
				}, {
					Name:           "account3-ap-southeast-1c-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "apac",
					Region:         "ap-southeast-1",
					AZ:             "ap-southeast-1c",
				}, {
					Name:           "account4-ap-northeast-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "apac",
					Region:         "ap-northeast-1",
					AZ:             "ap-northeast-1a",
				}, {
					Name:           "account4-ap-northeast-1a-2",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "apac",
					Region:         "ap-northeast-1",
					AZ:             "ap-northeast-1a",
				},
			}
			clusters, err := createClusters(ctx, targets)
			Expect(clusters).To(Not(BeNil()))
			Expect(err).To(BeNil())

			By("creating one application targeting each cluster")
			apps, err := createApplications(ctx, targets)
			var appsMap map[string]argov1alpha1.Application = make(map[string]argov1alpha1.Application)

			for i := 0; i < len(apps); i++ {
				appsMap[apps[i].Name] = apps[i]
			}

			Expect(apps).To(Not(BeNil()))
			Expect(err).To(BeNil())

			By("creating a progressive sync")
			ps := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-ps", testPrefix),
					Namespace: namespace,
				},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     consts.AppSetKind,
						Name:     appSet,
					},
					Stages: []syncv1alpha1.ProgressiveSyncStage{{
						Name:        "one cluster as canary in eu-west-1",
						MaxParallel: intstr.Parse("1"),
						MaxTargets:  intstr.Parse("1"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{MatchLabels: map[string]string{
									"region": "eu-west-1",
								}},
							}},
					}, {
						Name:        "one cluster as canary in every other region",
						MaxParallel: intstr.Parse("3"),
						MaxTargets:  intstr.Parse("3"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "region",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"eu-west-1"},
									}},
								},
							}},
					}, {
						Name:        "rollout to remaining clusters",
						MaxParallel: intstr.Parse("1"),
						MaxTargets:  intstr.Parse("4"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{},
							}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &ps)).To(Succeed())

			// Make sure the finalizer is added
			createdPS := syncv1alpha1.ProgressiveSync{}
			Eventually(func() int {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &createdPS)
				return len(createdPS.ObjectMeta.Finalizers)
			}).Should(Equal(1))
			Expect(createdPS.ObjectMeta.Finalizers[0]).To(Equal(syncv1alpha1.ProgressiveSyncFinalizer))

			psKey := client.ObjectKey{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-ps", testPrefix),
			}

			// Make sure that the first application is marked as synced in the first stage
			Eventually(func() bool {
				return isMarkedInStage(
					ps,
					appsMap["account1-eu-west-1a-1"],
					ps.Spec.Stages[0],
				)
			}).Should(BeTrue())

			// We need to progress account1-eu-west-1a-1 because the selector returns
			// a sorted-by-name list, so account1-eu-west-1a-1 will be the first one
			By("progressing account1-eu-west-1a-1")

			// Progress account1-eu-west-1a-1
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account1-eu-west-1a-1", namespace)
			}).Should(Succeed())

			// Make sure the stage is progressing
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: "one cluster as canary in eu-west-1 stage in progress",
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(1))

			By("completing account1-eu-west-1a-1 sync")

			// Completed account1-eu-west-1a-1
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account1-eu-west-1a-1", namespace)
			}).Should(Succeed())

			// Make sure the stage is completed
			message := "one cluster as canary in eu-west-1 stage completed"
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: message,
			}))

			// Make sure the ProgressiveSync status is still progressing because the sync is not completed yet
			progress := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			// The sort-by-name function will return
			// account2-eu-central-1a-1, account2-eu-central-1b-1
			// and account3-ap-southeast-1a-1
			By("progressing the second stage applications")

			// Progress the applications
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account2-eu-central-1a-1", namespace)
			}).Should(Succeed())
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account2-eu-central-1b-1", namespace)
			}).Should(Succeed())
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account3-ap-southeast-1a-1", namespace)
			}).Should(Succeed())

			Eventually(func() bool {
				return isMarkedInStage(
					ps,
					appsMap["account2-eu-central-1a-1"],
					ps.Spec.Stages[1],
				)
			}).Should(BeTrue())

			Eventually(func() bool {
				return isMarkedInStage(
					ps,
					appsMap["account2-eu-central-1b-1"],
					ps.Spec.Stages[1],
				)
			}).Should(BeTrue())

			Eventually(func() bool {
				return isMarkedInStage(
					ps,
					appsMap["account3-ap-southeast-1a-1"],
					ps.Spec.Stages[1],
				)
			}).Should(BeTrue())

			// Make sure the previous stage is still completed
			// TODO: we probably need to check that startedAt and finishedAt didn't change
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: message,
			}))

			// Make sure the current stage is progressing
			message = "one cluster as canary in every other region stage in progress"
			ExpectStageStatus(ctx, psKey, "one cluster as canary in every other region").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in every other region",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))

			// Make sure there are only two stages in status.stages
			// TODO: should we change ExpectStagesInStatus to take the stages name and return a bool?
			ExpectStagesInStatus(ctx, psKey).Should(Equal(2))

			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			progress = ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing the second stage applications sync")

			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account2-eu-central-1a-1", namespace)
			}).Should(Succeed())
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account2-eu-central-1b-1", namespace)
			}).Should(Succeed())
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account3-ap-southeast-1a-1", namespace)
			}).Should(Succeed())

			// Make sure the current stage is completed
			message = "one cluster as canary in every other region stage completed"
			ExpectStageStatus(ctx, psKey, "one cluster as canary in every other region").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in every other region",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: message,
			}))

			// Make sure the ProgressiveSync status is still progressing because the sync is not completed yet
			progress = ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			// The sort-by-name function will return
			// account1-eu-west-1a-2, account3-ap-southeast-1c-1
			// account4-ap-northeast-1a-1 and account4-ap-northeast-1a-2
			By("progressing 25% of the third stage applications")

			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account1-eu-west-1a-2", namespace)
			}).Should(Succeed())

			Eventually(func() bool {
				return isMarkedInStage(
					ps,
					appsMap["account1-eu-west-1a-2"],
					ps.Spec.Stages[2],
				)
			}).Should(BeTrue())

			// Make sure the previous stages are still completed
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "one cluster as canary in eu-west-1 stage completed",
			}))
			ExpectStageStatus(ctx, psKey, "one cluster as canary in every other region").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in every other region",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "one cluster as canary in every other region stage completed",
			}))

			// Make sure the current stage is progressing
			message = "rollout to remaining clusters stage in progress"
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))

			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			progress = ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing 25% of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account1-eu-west-1a-2", namespace)
			}).Should(Succeed())

			// Make sure the current stage is still in progress
			// because we still have 75% of clusters to sync
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))

			// Make sure the ProgressiveSync is still in progress
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("progressing 50% of the third stage applications")
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account3-ap-southeast-1c-1", namespace)
			}).Should(Succeed())

			Eventually(func() bool {
				return isMarkedInStage(
					ps,
					appsMap["account3-ap-southeast-1c-1"],
					ps.Spec.Stages[2],
				)
			}).Should(BeTrue())

			// Make sure the current stage is progressing
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))

			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing 50% of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account3-ap-southeast-1c-1", namespace)
			}).Should(Succeed())

			// Make sure the current stage is still in progress
			// because we still have 75% of clusters to sync
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))

			// Make sure the ProgressiveSync is still in progress
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("progressing 75% of the third stage applications")

			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account4-ap-northeast-1a-1", namespace)
			}).Should(Succeed())

			Eventually(func() bool {
				return isMarkedInStage(
					ps,
					appsMap["account4-ap-northeast-1a-1"],
					ps.Spec.Stages[2],
				)
			}).Should(BeTrue())

			// Make sure the current stage is progressing
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))

			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing 75% of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account4-ap-northeast-1a-1", namespace)
			}).Should(Succeed())

			// Make sure the current stage is still in progress
			// because we still have 75% of clusters to sync
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))

			// Make sure the ProgressiveSync is still in progress
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("progressing 100% of the third stage applications")

			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account4-ap-northeast-1a-2", namespace)
			}).Should(Succeed())

			Eventually(func() bool {
				return isMarkedInStage(
					ps,
					appsMap["account4-ap-northeast-1a-2"],
					ps.Spec.Stages[2],
				)
			}).Should(BeTrue())

			// Make sure the current stage is progressing
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))

			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing 100% of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account4-ap-northeast-1a-2", namespace)
			}).Should(Succeed())

			// Make sure the current stage is completed
			message = "rollout to remaining clusters stage completed"
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: message,
			}))

			// Make sure the ProgressiveSync is completed
			expected := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesCompleteReason, "All stages completed")
			ExpectCondition(&ps, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))

		})

		It("should fail if unable to sync an application", func() {
			testPrefix := "failed-stage"
			appSet := fmt.Sprintf("%s-appset", testPrefix)

			By("creating two ArgoCD clusters")
			targets := []Target{
				{
					Name:           "account5-us-west-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "na",
					Region:         "us-west-1",
					AZ:             "us-west-1a",
				}, {
					Name:           "account5-us-west-1a-2",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "na",
					Region:         "us-west-1",
					AZ:             "us-west-1a",
				},
			}
			clusters, err := createClusters(ctx, targets)
			Expect(err).To(BeNil())
			Expect(clusters).To(Not(BeNil()))

			By("creating one application targeting each cluster")
			apps, err := createApplications(ctx, targets)
			Expect(err).To(BeNil())
			Expect(apps).To(Not(BeNil()))

			By("creating a progressive sync")
			failedStagePS := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-ps", testPrefix), Namespace: namespace},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     consts.AppSetKind,
						Name:     appSet,
					},
					Stages: []syncv1alpha1.ProgressiveSyncStage{{
						Name:        "stage 0",
						MaxParallel: intstr.Parse("1"),
						MaxTargets:  intstr.Parse("1"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster": clusters[0].Name,
							}},
						}},
					}, {
						Name:        "stage 1",
						MaxParallel: intstr.Parse("1"),
						MaxTargets:  intstr.Parse("1"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster": clusters[1].Name,
							}},
						}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &failedStagePS)).To(Succeed())

			psKey := client.ObjectKey{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-ps", testPrefix),
			}

			By("progressing the first application")
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account5-us-west-1a-1", namespace)
			}).Should(Succeed())

			ExpectStageStatus(ctx, psKey, "stage 0").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "stage 0",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: "stage 0 stage in progress",
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(1))

			By("failing syncing the first application")
			Eventually(func() error {
				return setAppStatusFailed(ctx, "account5-us-west-1a-1", namespace)
			}).Should(Succeed())

			ExpectStageStatus(ctx, psKey, "stage 0").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "stage 0",
				Phase:   syncv1alpha1.PhaseFailed,
				Message: "stage 0 stage failed",
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(1))

			expected := failedStagePS.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesFailedReason, "stage 0 stage failed")
			ExpectCondition(&failedStagePS, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))
		})

		It("should set conditions when apps already synced - empty stage", func() {
			testPrefix := "synced-stage-empty"
			appSet := fmt.Sprintf("%s-appset", testPrefix)

			By("creating two ArgoCD clusters")
			targets := []Target{
				{
					Name:           "cells-1-eu-west-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1a",
				},
				{
					Name:           "cells-1-eu-west-1b-2",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1b-2",
				},
				{
					Name:           "cells-1-eu-west-1c-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1c-1",
				},
				{
					Name:           "cellsdev-1-eu-west-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1a-1",
				},
				{
					Name:           "cellsdev-1-eu-west-1b-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1b-1",
				},
			}
			clusters, err := createClusters(ctx, targets)
			Expect(err).To(BeNil())
			Expect(clusters).To(Not(BeNil()))

			By("creating one healthy and synced application targeting each cluster")
			apps, err := createSyncedAndHealthyApplications(ctx, targets)
			Expect(err).To(BeNil())
			Expect(apps).To(Not(BeNil()))

			By("creating a progressive sync")
			syncedStagePS := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-ps", testPrefix),
					Namespace: namespace,
				},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     consts.AppSetKind,
						Name:     appSet,
					},
					Stages: []syncv1alpha1.ProgressiveSyncStage{{
						Name:        "one cluster as canary in eu-west-1",
						MaxParallel: intstr.Parse("1"),
						MaxTargets:  intstr.Parse("1"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{MatchLabels: map[string]string{
									"region": "eu-west-1",
								}},
							}},
					}, {
						Name:        "one cluster as canary in every other region",
						MaxParallel: intstr.Parse("3"),
						MaxTargets:  intstr.Parse("3"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "region",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"eu-west-1"},
									}},
								},
							}},
					}, {
						Name:        "rollout to remaining clusters",
						MaxParallel: intstr.Parse("25%"),
						MaxTargets:  intstr.Parse("100%"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{},
							}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &syncedStagePS)).To(Succeed())

			psKey := client.ObjectKey{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-ps", testPrefix),
			}
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "one cluster as canary in eu-west-1 stage completed",
			}))
			ExpectStageStatus(ctx, psKey, "one cluster as canary in every other region").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in every other region",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "one cluster as canary in every other region stage completed",
			}))
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "rollout to remaining clusters stage completed",
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))

			expected := syncedStagePS.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesCompleteReason, "All stages completed")
			ExpectCondition(&syncedStagePS, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))

		})

		It("should reconcile a multi-stage progressive sync after controller restart", func() {
			testPrefix := "reset-multi"
			appSet := fmt.Sprintf("%s-appset", testPrefix)

			By("creating five ArgoCD clusters")
			targets := []Target{
				{
					Name:           "cells-1-eu-west-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1a",
				},
				{
					Name:           "cells-1-eu-west-1b-2",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1b-2",
				},
				{
					Name:           "cells-1-eu-west-1c-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1c-1",
				},
				{
					Name:           "cellsdev-1-eu-west-1a-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1a-1",
				},
				{
					Name:           "cellsdev-1-eu-west-1b-1",
					Namespace:      namespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1b-1",
				},
			}
			clusters, err := createClusters(ctx, targets)
			Expect(err).To(BeNil())
			Expect(clusters).To(Not(BeNil()))

			By("creating one application targeting each cluster")
			apps, err := createApplications(ctx, targets)
			Expect(apps).To(Not(BeNil()))
			Expect(err).To(BeNil())

			By("creating a progressive sync")
			ps := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-ps", testPrefix),
					Namespace: namespace,
				},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     consts.AppSetKind,
						Name:     appSet,
					},
					Stages: []syncv1alpha1.ProgressiveSyncStage{{
						Name:        "one cluster as canary in eu-west-1",
						MaxParallel: intstr.Parse("1"),
						MaxTargets:  intstr.Parse("1"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{MatchLabels: map[string]string{
									"region": "eu-west-1",
								}},
							}},
					}, {
						Name:        "one cluster as canary in every other region",
						MaxParallel: intstr.Parse("3"),
						MaxTargets:  intstr.Parse("3"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{{
										Key:      "region",
										Operator: metav1.LabelSelectorOpNotIn,
										Values:   []string{"eu-west-1"},
									}},
								},
							}},
					}, {
						Name:        "rollout to remaining clusters",
						MaxParallel: intstr.Parse("1"),
						MaxTargets:  intstr.Parse("4"),
						Targets: syncv1alpha1.ProgressiveSyncTargets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{},
							}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &ps)).To(Succeed())

			// Make sure the finalizer is added
			createdPS := syncv1alpha1.ProgressiveSync{}
			Eventually(func() int {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &createdPS)
				return len(createdPS.ObjectMeta.Finalizers)
			}).Should(Equal(1))
			Expect(createdPS.ObjectMeta.Finalizers[0]).To(Equal(syncv1alpha1.ProgressiveSyncFinalizer))

			psKey := client.ObjectKey{
				Namespace: namespace,
				Name:      fmt.Sprintf("%s-ps", testPrefix),
			}

			// We need to progress cells-1-eu-west-1a-1 because the selector returns
			// a sorted-by-name list, so cells-1-eu-west-1a-1 will be the first one
			By("progressing cells-1-eu-west-1a-1")
			// Progress account1-eu-west-1a-1
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "cells-1-eu-west-1a-1", namespace)
			}).Should(Succeed())
			// Make sure the stage is progressing
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: "one cluster as canary in eu-west-1 stage in progress",
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(1))

			By("completing cells-1-eu-west-1a-1 sync")
			// Completed cells-1-eu-west-1a-1
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "cells-1-eu-west-1a-1", namespace)
			}).Should(Succeed())
			// Make sure the stage is completed
			message := "one cluster as canary in eu-west-1 stage completed"
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: message,
			}))
			// Make sure the ProgressiveSync status is still progressing because the sync is not completed yet
			progress := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			// Stage 2 is be empty
			By("second stage is set straight to completed")
			// Make sure the previous stage is still completed
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: message,
			}))
			// Make sure the current stage is completed
			message = "one cluster as canary in every other region stage completed"
			ExpectStageStatus(ctx, psKey, "one cluster as canary in every other region").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in every other region",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: message,
			}))
			// Make sure there are only two stages in status.stages
			ExpectStagesInStatus(ctx, psKey).Should(Equal(2))
			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			progress = ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			// The sort-by-name function will return
			// cells-1-eu-west-1b-2, cells-1-eu-west-1c-1
			// cellsdev-1-eu-west-1a-1, cellsdev-1-eu-west-1b-1
			By("progressing 1/4 application of the third stage applications")
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "cells-1-eu-west-1b-2", namespace)
			}).Should(Succeed())
			// Make sure the previous stages are still completed
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "one cluster as canary in eu-west-1 stage completed",
			}))
			ExpectStageStatus(ctx, psKey, "one cluster as canary in every other region").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in every other region",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "one cluster as canary in every other region stage completed",
			}))
			// Make sure the current stage is progressing
			message = "rollout to remaining clusters stage in progress"
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))
			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			progress = ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionFalse, syncv1alpha1.StagesProgressingReason, message)
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing 1/4 of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "cells-1-eu-west-1b-2", namespace)
			}).Should(Succeed())
			// Make sure the current stage is still in progress
			// because we still have 3 applications from the clusters to sync
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			// Make sure the ProgressiveSync is still in progress
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			ResetController()

			By("progressing 2/4 of the third stage applications")
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "cells-1-eu-west-1c-1", namespace)
			}).Should(Succeed())
			// Make sure the previous stages are still completed
			ExpectStageStatus(ctx, psKey, "one cluster as canary in eu-west-1").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in eu-west-1",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "one cluster as canary in eu-west-1 stage completed",
			}))
			ExpectStageStatus(ctx, psKey, "one cluster as canary in every other region").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "one cluster as canary in every other region",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: "one cluster as canary in every other region stage completed",
			}))
			// Make sure the current stage is progressing
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))
			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing 2/4 of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "cells-1-eu-west-1c-1", namespace)
			}).Should(Succeed())
			// Make sure the current stage is still in progress
			// because we still have 2 applications from the clusters to sync
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			// Make sure the ProgressiveSync is still in progress
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("progressing 3/4 of the third stage applications")
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "cellsdev-1-eu-west-1a-1", namespace)
			}).Should(Succeed())
			// Make sure the current stage is progressing
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))
			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing 3/4 of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "cellsdev-1-eu-west-1a-1", namespace)
			}).Should(Succeed())
			// Make sure the current stage is still in progress
			// because we still have 1 application from the clusters to sync
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			// Make sure the ProgressiveSync is still in progress
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("progressing 4/4 of the third stage applications")
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "cellsdev-1-eu-west-1b-1", namespace)
			}).Should(Succeed())
			// Make sure the current stage is progressing
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseProgressing,
				Message: message,
			}))
			ExpectStagesInStatus(ctx, psKey).Should(Equal(3))
			// Make sure the ProgressiveSync status is progressing because the sync is not completed yet
			ExpectCondition(&ps, progress.Type).Should(HaveStatus(progress.Status, progress.Reason, progress.Message))

			By("completing 4/4 of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "cellsdev-1-eu-west-1b-1", namespace)
			}).Should(Succeed())
			// Make sure the current stage is completed
			message = "rollout to remaining clusters stage completed"
			ExpectStageStatus(ctx, psKey, "rollout to remaining clusters").Should(MatchStage(syncv1alpha1.StageStatus{
				Name:    "rollout to remaining clusters",
				Phase:   syncv1alpha1.PhaseSucceeded,
				Message: message,
			}))
			// Make sure the ProgressiveSync is completed
			expected := ps.NewStatusCondition(syncv1alpha1.CompletedCondition, metav1.ConditionTrue, syncv1alpha1.StagesCompleteReason, "All stages completed")
			ExpectCondition(&ps, expected.Type).Should(HaveStatus(expected.Status, expected.Reason, expected.Message))
		})
	})

	Describe("sync application", func() {
		It("should send a request to sync an application", func() {
			mockedArgoCDAppClient := mocks.MockArgoCDAppClientCalledWith{}
			reconciler.ArgoCDAppClient = &mockedArgoCDAppClient
			testAppName := "single-stage-app"

			By("creating an ArgoCD cluster")
			cluster := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-cluster", Namespace: namespace, Labels: map[string]string{consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster}},
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
						APIVersion: consts.AppSetAPIGroup,
						Kind:       consts.AppSetKind,
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
						Kind:     consts.AppSetKind,
						Name:     "single-stage-appset",
					},
					Stages: []syncv1alpha1.ProgressiveSyncStage{{
						Name:        "stage 1",
						MaxParallel: intstr.Parse("1"),
						MaxTargets:  intstr.Parse("1"),
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
func createClusters(ctx context.Context, targets []Target) ([]corev1.Secret, error) {
	var clusters []corev1.Secret

	for _, t := range targets {
		cluster := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      t.Name,
				Namespace: t.Namespace,
				Labels: map[string]string{
					consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster,
					"area":                       t.Area,
					"region":                     t.Region,
					"az":                         t.AZ,
					"cluster":                    t.Name,
				}},
			Data: map[string][]byte{
				"server": []byte(fmt.Sprintf("https://%s.kubernetes.io", t.Name)),
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
func createApplications(ctx context.Context, targets []Target) ([]argov1alpha1.Application, error) {
	var apps []argov1alpha1.Application

	for _, t := range targets {

		app := argov1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      t.Name,
				Namespace: t.Namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: consts.AppSetAPIGroup,
					Kind:       consts.AppSetKind,
					Name:       t.ApplicationSet,
					UID:        uuid.NewUUID(),
				}},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Destination: argov1alpha1.ApplicationDestination{
					Server:    fmt.Sprintf("https://%s.kubernetes.io", t.Name),
					Namespace: t.Namespace,
					Name:      t.Name,
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

		apps = append(apps, app)
	}

	return apps, nil
}

// createSyncedAndHealthyApplications is a helper function that creates an ArgoCD application given a prefix and a cluster
func createSyncedAndHealthyApplications(ctx context.Context, targets []Target) ([]argov1alpha1.Application, error) {
	var apps []argov1alpha1.Application

	for _, t := range targets {

		app := argov1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      t.Name,
				Namespace: t.Namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: consts.AppSetAPIGroup,
					Kind:       consts.AppSetKind,
					Name:       t.ApplicationSet,
					UID:        uuid.NewUUID(),
				}},
			},
			Spec: argov1alpha1.ApplicationSpec{
				Destination: argov1alpha1.ApplicationDestination{
					Server:    fmt.Sprintf("https://%s.kubernetes.io", t.Name),
					Namespace: t.Namespace,
					Name:      t.Name,
				}},
			Status: argov1alpha1.ApplicationStatus{
				Sync: argov1alpha1.SyncStatus{
					Status: argov1alpha1.SyncStatusCodeSynced,
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

		apps = append(apps, app)
	}

	return apps, nil
}

// setAppStatusProgressing set the application health status to progressing given an application name and its namespace
func setAppStatusProgressing(ctx context.Context, appName string, namespace string) error {
	app := argov1alpha1.Application{}
	err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      appName,
	}, &app)

	if err != nil {
		return err
	}

	app.Status.Health = argov1alpha1.HealthStatus{
		Status:  health.HealthStatusProgressing,
		Message: "progressing",
	}

	return k8sClient.Update(ctx, &app)
}

// setAppStatusCompleted set the application health status to completed given an application name and its namespace
func setAppStatusCompleted(ctx context.Context, appName string, namespace string) error {
	app := argov1alpha1.Application{}
	err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      appName,
	}, &app)

	if err != nil {
		return err
	}

	app.Status.Health = argov1alpha1.HealthStatus{
		Status:  health.HealthStatusHealthy,
		Message: "healthy",
	}
	app.Status.Sync = argov1alpha1.SyncStatus{
		Status: argov1alpha1.SyncStatusCodeSynced,
	}

	return k8sClient.Update(ctx, &app)
}

// setAppStatusFailed set the application health status to failed given an application name and its namespace
func setAppStatusFailed(ctx context.Context, appName string, namespace string) error {
	app := argov1alpha1.Application{}
	err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      appName,
	}, &app)

	if err != nil {
		return err
	}

	app.Status.Health = argov1alpha1.HealthStatus{
		Status:  health.HealthStatusDegraded,
		Message: "degraded",
	}
	app.Status.Sync = argov1alpha1.SyncStatus{
		Status: argov1alpha1.SyncStatusCodeSynced,
	}

	return k8sClient.Update(ctx, &app)
}

func ResetController() {
	reconciler.StateManager = utils.NewProgressiveSyncManager()
}

// isMarkedInStage returns true if the application has been marked as synced in the specified stage
func isMarkedInStage(ps syncv1alpha1.ProgressiveSync, app argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) bool {

	pss, _ := reconciler.StateManager.Get(ps.Name)
	return pss.IsAppMarkedInStage(app, stage)

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
	ps *syncv1alpha1.ProgressiveSync, ct string,
) AsyncAssertion {
	return Eventually(func() string {
		_ = k8sClient.Get(
			context.Background(),
			types.NamespacedName{Name: ps.Name, Namespace: ps.Namespace},
			ps,
		)
		for _, c := range ps.Status.Conditions {
			if c.Type == ct {
				return statusString(c.Status, c.Reason, c.Message)
			}
		}
		return ""
	})
}

// ExpectStageStatus returns an AsyncAssertion for a StageStatus, given a ProgressiveSync object key and a stage name
func ExpectStageStatus(ctx context.Context, key client.ObjectKey, stageName string) AsyncAssertion {
	return Eventually(func() syncv1alpha1.StageStatus {
		ps := syncv1alpha1.ProgressiveSync{}
		err := k8sClient.Get(ctx, key, &ps)
		if err != nil {
			return syncv1alpha1.StageStatus{}
		}
		stageStatus := syncv1alpha1.FindStageStatus(ps.Status.Stages, stageName)
		if stageStatus != nil {
			return *stageStatus
		}

		return syncv1alpha1.StageStatus{}
	})
}

// ExpectStagesInStatus returns an AsyncAssertion for the length of stages with status in the Progressive Rollout object
func ExpectStagesInStatus(ctx context.Context, key client.ObjectKey) AsyncAssertion {
	return Eventually(func() int {
		ps := syncv1alpha1.ProgressiveSync{}
		err := k8sClient.Get(ctx, key, &ps)
		if err != nil {
			return -1
		}

		return len(ps.Status.Stages)
	})
}

func TestSync(t *testing.T) {
	r := ProgressiveSyncReconciler{
		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncOK{},
		StateManager:    utils.NewProgressiveSyncManager(),
	}

	testAppName := "foo-bar"

	application, err := r.syncApp(ctx, testAppName)

	g := NewWithT(t)
	g.Expect(err).To(BeNil())
	g.Expect(application.Name).To(Equal(testAppName))
}

func TestSyncErr(t *testing.T) {
	r := ProgressiveSyncReconciler{
		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncNotOK{},
		StateManager:    utils.NewProgressiveSyncManager(),
	}

	testAppName := "foo-bar"

	application, err := r.syncApp(ctx, testAppName)

	g := NewWithT(t)
	g.Expect(application).To(BeNil())
	g.Expect(err).ToNot(BeNil())
}

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	"github.com/Skyscanner/applicationset-progressive-sync/mocks"
	applicationset "github.com/argoproj-labs/applicationset/api/v1alpha1"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	pkgmeta "github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	timeout  = time.Second * 60
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
		ctrlNamespace string
		ns            *corev1.Namespace
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		ctrlNamespace, ns = createRandomNamespace()

		k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:   scheme.Scheme,
			NewCache: cache.MultiNamespacedCacheBuilder([]string{ctrlNamespace, argoNamespace}),
		})
		Expect(err).ToNot(HaveOccurred())

		mockAcdClient := mocks.ArgoCDAppClientStub{}
		reconciler = &ProgressiveSyncReconciler{
			Client:          k8sManager.GetClient(),
			Scheme:          k8sManager.GetScheme(),
			ArgoCDAppClient: &mockAcdClient,
			ArgoNamespace:   argoNamespace,
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
			localNS := argoNamespace
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
			ownerPR := createOwnerPR(ctrlNamespace, "owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			nonOwnedApp := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-owned-app",
					Namespace: argoNamespace,
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
			ownerPR := createOwnerPR(ctrlNamespace, "secret-owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			serverURL := "https://kubernetes.default.svc"

			By("creating an application")
			app := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "matching-app",
					Namespace: argoNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: consts.AppSetAPIGroup,
						Kind:       consts.AppSetKind,
						Name:       "owner-app-set",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
					Server:    serverURL,
					Namespace: argoNamespace,
					Name:      "local-cluster",
				}},
			}
			Expect(k8sClient.Create(ctx, &app)).To(Succeed())

			By("creating a cluster secret")
			cluster := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: argoNamespace, Labels: map[string]string{consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster}},
				Data:       map[string][]byte{"server": []byte(serverURL)}}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(&cluster)
				return len(requests)
			}).Should(Equal(1))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})

		It("should not forward an event for a generic secret", func() {
			ownerPR := createOwnerPR(ctrlNamespace, "generic-owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())
			By("creating a generic secret")
			generic := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "generic", Namespace: argoNamespace}, Data: map[string][]byte{"secret": []byte("insecure")},
			}

			Eventually(func() int {
				requests := reconciler.requestsForSecretChange(&generic)
				return len(requests)
			}).Should(Equal(0))
			Expect(k8sClient.Delete(ctx, ownerPR)).To(Succeed())
		})

		It("should not forward an event for an argocd secret not matching any application", func() {
			ownerPR := createOwnerPR(ctrlNamespace, "owner-pr")
			Expect(k8sClient.Create(ctx, ownerPR)).To(Succeed())

			By("creating an application")
			externalApp := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-matching-app",
					Namespace: argoNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: consts.AppSetAPIGroup,
						Kind:       consts.AppSetKind,
						Name:       "owner-app-set",
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://remote-url.kubernetes.io",
					Namespace: argoNamespace,
					Name:      "remote-cluster",
				}},
			}
			Expect(k8sClient.Create(ctx, &externalApp)).To(Succeed())

			By("creating a cluster secret")
			internalCluster := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: argoNamespace, Labels: map[string]string{consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster}},
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
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1a",
				}, {
					Name:           "account1-eu-west-1a-2",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-west-1",
					AZ:             "eu-west-1a",
				}, {
					Name:           "account2-eu-central-1a-1",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-central-1",
					AZ:             "eu-central-1a",
				}, {
					Name:           "account2-eu-central-1b-1",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "emea",
					Region:         "eu-central-1",
					AZ:             "eu-central-1b",
				}, {
					Name:           "account3-ap-southeast-1a-1",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "apac",
					Region:         "ap-southeast-1",
					AZ:             "ap-southeast-1a",
				}, {
					Name:           "account3-ap-southeast-1c-1",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "apac",
					Region:         "ap-southeast-1",
					AZ:             "ap-southeast-1c",
				}, {
					Name:           "account4-ap-northeast-1a-1",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "apac",
					Region:         "ap-northeast-1",
					AZ:             "ap-northeast-1a",
				}, {
					Name:           "account4-ap-northeast-1a-2",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "apac",
					Region:         "ap-northeast-1",
					AZ:             "ap-northeast-1a",
				},
			}
			clusters, err := createClusters(ctx, targets)
			Expect(clusters).To(Not(BeNil()))
			Expect(err).To(BeNil())

			By("creating an ApplicationSet")
			applicationSet, err := createApplicationSetWithLabels(
				ctx,
				appSet,
				argoNamespace,
				map[string]string{"foo": "bar"},
			)
			Expect(applicationSet).To(Not(BeNil()))
			Expect(err).To(BeNil())

			By("creating one application targeting each cluster")
			apps, err := createApplications(ctx, targets)
			Expect(apps).To(Not(BeNil()))
			Expect(err).To(BeNil())

			By("creating a progressive sync object")
			ps := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-ps", testPrefix),
					Namespace: ctrlNamespace,
				},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     consts.AppSetKind,
						Name:     appSet,
					},
					Stages: []syncv1alpha1.Stage{{
						Name:        "one cluster as canary in eu-west-1",
						MaxParallel: 1,
						MaxTargets:  1,
						Targets: syncv1alpha1.Targets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{MatchLabels: map[string]string{
									"region": "eu-west-1",
								}},
							}},
					}, {
						Name:        "one cluster as canary in every other region",
						MaxParallel: 3,
						MaxTargets:  3,
						Targets: syncv1alpha1.Targets{
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
						MaxParallel: 1,
						MaxTargets:  4,
						Targets: syncv1alpha1.Targets{
							Clusters: syncv1alpha1.Clusters{
								Selector: metav1.LabelSelector{},
							}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &ps)).To(Succeed())

			// We need to progress account1-eu-west-1a-1 because the selector returns
			// a sorted-by-name list, so account1-eu-west-1a-1 will be the first one
			By("progressing account1-eu-west-1a-1")

			// Progress account1-eu-west-1a-1
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account1-eu-west-1a-1", argoNamespace)
			}).Should(Succeed())

			Eventually(func() bool {
				var latest syncv1alpha1.ProgressiveSync
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &latest); err != nil {
					return false
				}
				return latest.Status.LastSyncedStage == "one cluster as canary in eu-west-1" && latest.Status.LastSyncedStageStatus == syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing)
			}).Should(BeTrue())

			By("completing account1-eu-west-1a-1 sync")

			// Completed account1-eu-west-1a-1
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account1-eu-west-1a-1", argoNamespace)
			}).Should(Succeed())

			// The sort-by-name function will return
			// account2-eu-central-1a-1, account2-eu-central-1b-1
			// and account3-ap-southeast-1a-1
			By("progressing the second stage applications")

			// Progress the applications
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account2-eu-central-1a-1", argoNamespace)
			}).Should(Succeed())
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account2-eu-central-1b-1", argoNamespace)
			}).Should(Succeed())
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account3-ap-southeast-1a-1", argoNamespace)
			}).Should(Succeed())

			Eventually(func() bool {
				var latest syncv1alpha1.ProgressiveSync
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &latest); err != nil {
					return false
				}
				return latest.Status.LastSyncedStage == "one cluster as canary in every other region" && latest.Status.LastSyncedStageStatus == syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing)
			}).Should(BeTrue())

			By("completing the second stage applications sync")

			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account2-eu-central-1a-1", argoNamespace)
			}).Should(Succeed())
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account2-eu-central-1b-1", argoNamespace)
			}).Should(Succeed())
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account3-ap-southeast-1a-1", argoNamespace)
			}).Should(Succeed())

			// The sort-by-name function will return
			// account1-eu-west-1a-2, account3-ap-southeast-1c-1
			// account4-ap-northeast-1a-1 and account4-ap-northeast-1a-2
			By("progressing 25% of the third stage applications")

			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account1-eu-west-1a-2", argoNamespace)
			}).Should(Succeed())

			Eventually(func() bool {
				var latest syncv1alpha1.ProgressiveSync
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &latest); err != nil {
					return false
				}
				return latest.Status.LastSyncedStage == "rollout to remaining clusters" && latest.Status.LastSyncedStageStatus == syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing)
			}).Should(BeTrue())

			By("completing 25% of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account1-eu-west-1a-2", argoNamespace)
			}).Should(Succeed())

			By("progressing 50% of the third stage applications")
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account3-ap-southeast-1c-1", argoNamespace)
			}).Should(Succeed())

			By("completing 50% of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account3-ap-southeast-1c-1", argoNamespace)
			}).Should(Succeed())

			By("progressing 75% of the third stage applications")

			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account4-ap-northeast-1a-1", argoNamespace)
			}).Should(Succeed())

			By("completing 75% of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account4-ap-northeast-1a-1", argoNamespace)
			}).Should(Succeed())

			By("progressing 100% of the third stage applications")

			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account4-ap-northeast-1a-2", argoNamespace)
			}).Should(Succeed())

			By("completing 100% of the third stage applications")
			Eventually(func() error {
				return setAppStatusCompleted(ctx, "account4-ap-northeast-1a-2", argoNamespace)
			}).Should(Succeed())

			Eventually(func() bool {
				var latest syncv1alpha1.ProgressiveSync
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &latest); err != nil {
					return false
				}
				return latest.Status.LastSyncedStage == "rollout to remaining clusters" && latest.Status.LastSyncedStageStatus == syncv1alpha1.StageStatus(syncv1alpha1.StageStatusCompleted)
			}).Should(BeTrue())

			Eventually(func() bool {
				var latest syncv1alpha1.ProgressiveSync
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&ps), &latest); err != nil {
					return false
				}
				readyCondition := meta.FindStatusCondition(latest.Status.Conditions, pkgmeta.ReadyCondition)
				if readyCondition == nil {
					return false
				}
				if readyCondition.Type == pkgmeta.ReadyCondition && readyCondition.Status == metav1.ConditionTrue && readyCondition.Reason == pkgmeta.ReconciliationSucceededReason {
					return true
				}
				return false
			}).Should(BeTrue())
		})

		It("should fail if unable to sync an application", func() {
			testPrefix := "failed-stage"
			appSet := fmt.Sprintf("%s-appset", testPrefix)

			By("creating two ArgoCD clusters")
			targets := []Target{
				{
					Name:           "account5-us-west-1a-1",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "na",
					Region:         "us-west-1",
					AZ:             "us-west-1a",
				}, {
					Name:           "account5-us-west-1a-2",
					Namespace:      argoNamespace,
					ApplicationSet: appSet,
					Area:           "na",
					Region:         "us-west-1",
					AZ:             "us-west-1a",
				},
			}
			clusters, err := createClusters(ctx, targets)
			Expect(err).To(BeNil())
			Expect(clusters).To(Not(BeNil()))

			By("creating an ApplicationSet")
			applicationSet, err := createApplicationSetWithLabels(
				ctx,
				appSet,
				argoNamespace,
				map[string]string{"foo": "bar"},
			)
			Expect(applicationSet).To(Not(BeNil()))
			Expect(err).To(BeNil())

			By("creating one application targeting each cluster")
			apps, err := createApplications(ctx, targets)
			Expect(err).To(BeNil())
			Expect(apps).To(Not(BeNil()))

			By("creating a progressive sync object")
			failedStagePS := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-ps", testPrefix), Namespace: ctrlNamespace},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     consts.AppSetKind,
						Name:     appSet,
					},
					Stages: []syncv1alpha1.Stage{{
						Name:        "stage 0",
						MaxParallel: 1,
						MaxTargets:  1,
						Targets: syncv1alpha1.Targets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster": clusters[0].Name,
							}},
						}},
					}, {
						Name:        "stage 1",
						MaxParallel: 1,
						MaxTargets:  1,
						Targets: syncv1alpha1.Targets{Clusters: syncv1alpha1.Clusters{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"cluster": clusters[1].Name,
							}},
						}},
					}},
				},
			}
			Expect(k8sClient.Create(ctx, &failedStagePS)).To(Succeed())

			By("progressing the first application")
			Eventually(func() error {
				return setAppStatusProgressing(ctx, "account5-us-west-1a-1", argoNamespace)
			}).Should(Succeed())

			By("failing syncing the first application")
			Eventually(func() error {
				return setAppStatusFailed(ctx, "account5-us-west-1a-1", argoNamespace)
			}).Should(Succeed())

			Eventually(func() bool {
				var latest syncv1alpha1.ProgressiveSync
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&failedStagePS), &latest); err != nil {
					return false
				}
				return latest.Status.LastSyncedStage == "stage 0" && latest.Status.LastSyncedStageStatus == syncv1alpha1.StageStatus(syncv1alpha1.StageStatusFailed)
			}).Should(BeTrue())

			Eventually(func() bool {
				var latest syncv1alpha1.ProgressiveSync
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&failedStagePS), &latest); err != nil {
					return false
				}
				readyCondition := meta.FindStatusCondition(latest.Status.Conditions, pkgmeta.ReadyCondition)
				if readyCondition == nil {
					return false
				}
				if readyCondition.Type == pkgmeta.ReadyCondition && readyCondition.Status == metav1.ConditionFalse && readyCondition.Reason == syncv1alpha1.StageFailedReason {
					return true
				}
				return false
			}).Should(BeTrue())
		})
	})

	Describe("sync application", func() {
		It("should send a request to sync an application", func() {
			mockedArgoCDAppClient := mocks.MockArgoCDAppClientCalledWith{}
			reconciler.ArgoCDAppClient = &mockedArgoCDAppClient
			testAppName := "single-stage-app"
			appSet := "single-stage-appset"

			By("creating an ArgoCD cluster")
			cluster := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-cluster", Namespace: argoNamespace, Labels: map[string]string{consts.ArgoCDSecretTypeLabel: consts.ArgoCDSecretTypeCluster}},
				Data: map[string][]byte{
					"server": []byte("https://single-stage-pr.kubernetes.io"),
				},
			}
			Expect(k8sClient.Create(ctx, &cluster)).To(Succeed())

			By("creating an ApplicationSet")
			applicationSet, err := createApplicationSetWithLabels(
				ctx,
				appSet,
				argoNamespace,
				map[string]string{"foo": "bar"},
			)
			Expect(applicationSet).To(Not(BeNil()))
			Expect(err).To(BeNil())

			By("creating an application targeting the cluster")
			singleStageApp := argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAppName,
					Namespace: argoNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: consts.AppSetAPIGroup,
						Kind:       consts.AppSetKind,
						Name:       appSet,
						UID:        uuid.NewUUID(),
					}},
				},
				Spec: argov1alpha1.ApplicationSpec{Destination: argov1alpha1.ApplicationDestination{
					Server:    "https://single-stage-pr.kubernetes.io",
					Namespace: argoNamespace,
					Name:      "remote-cluster",
				}},
				Status: argov1alpha1.ApplicationStatus{Sync: argov1alpha1.SyncStatus{Status: argov1alpha1.SyncStatusCodeOutOfSync}},
			}
			Expect(k8sClient.Create(ctx, &singleStageApp)).To(Succeed())

			By("creating a progressive sync")
			singleStagePR := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{Name: "single-stage-pr", Namespace: ctrlNamespace},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     consts.AppSetKind,
						Name:     appSet,
					},
					Stages: []syncv1alpha1.Stage{{
						Name:        "stage 1",
						MaxParallel: 1,
						MaxTargets:  1,
						Targets: syncv1alpha1.Targets{Clusters: syncv1alpha1.Clusters{
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

	Describe("calculate hash for applicationSet spec", func() {
		It("should have a different hash value when spec changes", func() {
			testPrefix := "hash-should-chage"
			psName := fmt.Sprintf("%s-ps", testPrefix)
			appSetName := fmt.Sprintf("%s-appset", testPrefix)

			By("creating a ProgressiveSync obj")
			ps := syncv1alpha1.ProgressiveSync{
				ObjectMeta: metav1.ObjectMeta{
					Name:      psName,
					Namespace: ctrlNamespace,
				},
				Spec: syncv1alpha1.ProgressiveSyncSpec{
					SourceRef: corev1.TypedLocalObjectReference{
						APIGroup: &appSetAPIRef,
						Kind:     consts.AppSetKind,
						Name:     appSetName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, &ps)).To(Succeed())

			By("creating an ApplicationSet obj")
			appSet, err := createApplicationSetWithLabels(
				ctx,
				appSetName,
				argoNamespace,
				map[string]string{"foo": "bar"},
			)
			Expect(appSet).To(Not(BeNil()))
			Expect(err).To(BeNil())

			// By("calculate current hash value")
			// hashedSpec, err := reconciler.calculateHashedSpec(ctx, &ps)
			// Expect(hashedSpec).To(Not(BeNil()))
			// Expect(err).To(BeNil())

			By("update the spec of the ApplicationSet obj")
			err = updateApplicationSetLabels(
				ctx,
				appSetName,
				argoNamespace,
				map[string]string{"new_label": "mock_value"},
			)
			Expect(err).To(BeNil())

			Eventually(func() error {
				appSet := applicationset.ApplicationSet{}
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: argoNamespace,
					Name:      appSetName,
				}, &appSet)
				if err != nil {
					return err
				}
				value, ok := appSet.Spec.Template.ApplicationSetTemplateMeta.Labels["new_label"]

				if !ok {
					return errors.New("The updated ApplicationSet spec is missing the new label")
				}

				if value != "mock_value" {
					return errors.New("The updated ApplicationSet spec is missing the new label value")
				}

				return nil
			}).Should(Succeed())

			// By("calculate the new hash value")
			// newHashedSpec, err := reconciler.calculateHashedSpec(ctx, &ps)
			// Expect(hashedSpec).To(Not(BeNil()))
			// Expect(err).To(BeNil())

			// Expect(hashedSpec).ToNot(Equal(newHashedSpec))
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

// createApplicationSet creates an ApplicationSet object with labels for the Application template
func createApplicationSetWithLabels(ctx context.Context, name string, namespace string, labels map[string]string) (applicationset.ApplicationSet, error) {
	applicationSet := applicationset.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: applicationset.ApplicationSetSpec{
			Template: applicationset.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: applicationset.ApplicationSetTemplateMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    labels,
				},
			},
			Generators: []applicationset.ApplicationSetGenerator{{}},
		},
	}

	if err := k8sClient.Create(ctx, &applicationSet); err != nil {
		return applicationset.ApplicationSet{}, err
	}

	return applicationSet, nil
}

// updateApplicationSetAnnotations updates the labels of the ApplicationSet spec
func updateApplicationSetLabels(ctx context.Context, appSetName string, namespace string, labels map[string]string) error {
	appSet := applicationset.ApplicationSet{}
	err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      appSetName,
	}, &appSet)

	if err != nil {
		return err
	}

	appSet.Spec.Template.Labels = labels

	return k8sClient.Update(ctx, &appSet)
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

	app.Status.Sync = argov1alpha1.SyncStatus{
		Status: argov1alpha1.SyncStatusCodeSynced,
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
// func ExpectStageStatus(ctx context.Context, key client.ObjectKey, stageName string) AsyncAssertion {
// 	return Eventually(func() syncv1alpha1.StageStatus {

// 		return syncv1alpha1.StageStatus(syncv1alpha1.StageStatusProgressing)
// 	})
// }

// ExpectStagesInStatus returns an AsyncAssertion for the length of stages with status in the Progressive Rollout object
// func ExpectStagesInStatus(ctx context.Context, key client.ObjectKey) AsyncAssertion {
// 	return Eventually(func() int {
// 		ps := syncv1alpha1.ProgressiveSync{}
// 		err := k8sClient.Get(ctx, key, &ps)
// 		if err != nil {
// 			return -1
// 		}

// 		return len(ps.Status.Stages)
// 	})
// }

// func TestSync(t *testing.T) {
// 	r := ProgressiveSyncReconciler{
// 		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncOK{},
// 	}

// 	testAppName := "foo-bar"

// 	application, err := r.syncApp(ctx, testAppName)

// 	g := NewWithT(t)
// 	g.Expect(err).To(BeNil())
// 	g.Expect(application.Name).To(Equal(testAppName))
// }

// func TestSyncErr(t *testing.T) {
// 	r := ProgressiveSyncReconciler{
// 		ArgoCDAppClient: &mocks.MockArgoCDAppClientSyncNotOK{},
// 		StateManager:    utils.NewProgressiveSyncManager(),
// 	}

// 	testAppName := "foo-bar"

// 	application, err := r.syncApp(ctx, testAppName)

// 	g := NewWithT(t)
// 	g.Expect(application).To(BeNil())
// 	g.Expect(err).ToNot(BeNil())
// }

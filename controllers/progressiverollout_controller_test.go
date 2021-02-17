package controllers

import (
	"context"
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"time"
)

const (
	timeout  = time.Second * 20
	interval = time.Millisecond * 100
)

var _ = Describe("ProgressiveRollout Controller", func() {

	apiGroup := "argoproj.io/v1alpha1"
	ctx := context.Background()
	// See https://onsi.github.io/gomega#modifying-default-intervals
	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(interval)

	BeforeEach(func() {
		namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "argocd"},
		}
		err := k8sClient.Create(context.Background(), &namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")
	})

	// We should replace this test with better ones as we deploy the controller
	It("should reconcile", func() {
		pr := deploymentskyscannernetv1alpha1.ProgressiveRollout{
			ObjectMeta: metav1.ObjectMeta{Name: "go-infrabin", Namespace: "argocd"},
			Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
				SourceRef: corev1.TypedLocalObjectReference{
					APIGroup: &apiGroup,
					Kind:     "ApplicationSet",
					Name:     "go-infra",
				},
				Stages: []*deploymentskyscannernetv1alpha1.ProgressiveRolloutStage{{
					Name:        "stage 1",
					MaxParallel: intstr.IntOrString{IntVal: 1},
					MaxTargets:  intstr.IntOrString{IntVal: 1},
					Targets: deploymentskyscannernetv1alpha1.ProgressiveRolloutTargets{Clusters: deploymentskyscannernetv1alpha1.Cluster{
						Selector: metav1.LabelSelector{MatchLabels: nil},
					}},
				}},
			},
		}
		Expect(k8sClient.Create(ctx, &pr)).To(Succeed())
		Eventually(func() string {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Namespace: pr.Namespace,
				Name:      pr.Name,
			}, &pr)
			return pr.Status.Conditions[0].Type
		}).Should(Equal(deploymentskyscannernetv1alpha1.CompletedCondition))
	})
})

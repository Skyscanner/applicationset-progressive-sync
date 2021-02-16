package controllers

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
)

const timeout = time.Second * 7

var _ = Describe("ProgressiveRollout Controller", func() {

	apiGroup := "argoproj.io/v1alpha1"
	ctx := context.Background()
	SetDefaultEventuallyTimeout(timeout)

	It("should ", func() {
		pr := &deploymentskyscannernetv1alpha1.ProgressiveRollout{
			ObjectMeta: metav1.ObjectMeta{Name: "go-infrabin", Namespace: "argocd"},
			Spec: deploymentskyscannernetv1alpha1.ProgressiveRolloutSpec{
				SourceRef: corev1.TypedLocalObjectReference{
					APIGroup: &apiGroup,
					Kind:     "ApplicationSet",
					Name:     "go-infra",
				},
			},
		}
		Expect(k8sClient.Create(ctx, pr)).To(Succeed())
		//Eventually(func() bool {
		//	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pr.Name, Namespace: pr.Namespace}, pr)).To(Succeed())
		//	return len(pr.Status.Conditions) > 0
		//}).Should(Equal(true))
	})
})

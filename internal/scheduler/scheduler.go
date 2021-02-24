package scheduler

import (
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
)

func Scheduler(clusters corev1.SecretList, apps []argov1alpha1.Application, stage deploymentskyscannernetv1alpha1.ProgressiveRolloutStage) []string {
	var scheduledApps []string
	return scheduledApps
}

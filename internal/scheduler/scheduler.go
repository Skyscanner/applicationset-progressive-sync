package scheduler

import (
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	"github.com/Skyscanner/argocd-progressive-rollout/internal/utils"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Scheduler returns a list of apps to sync
func Scheduler(apps []argov1alpha1.Application, stage deploymentskyscannernetv1alpha1.ProgressiveRolloutStage) []string {

	var scheduledApps []string

	oufOfSyncApps := utils.GetAppsBySyncStatusCode(apps, argov1alpha1.SyncStatusCodeOutOfSync)
	progressingApps := utils.GetAppsByHealthStatusCode(apps, health.HealthStatusProgressing)

	maxTargets, err := intstr.GetScaledValueFromIntOrPercent(&stage.MaxTargets, len(oufOfSyncApps), false)
	if err != nil {
		return scheduledApps
	}
	maxParallel, err := intstr.GetScaledValueFromIntOrPercent(&stage.MaxParallel, maxTargets, false)
	if err != nil {
		return scheduledApps
	}

	if len(oufOfSyncApps) > 0 {
		for i := 0; i < maxParallel-len(progressingApps); i++ {
			scheduledApps = append(scheduledApps, oufOfSyncApps[i].Name)
		}
	}
	return scheduledApps
}

func IsStageComplete(apps []argov1alpha1.Application, stage deploymentskyscannernetv1alpha1.ProgressiveRolloutStage) bool {
	//TODO: add logic
	return true
}

func IsStageFailed(apps []argov1alpha1.Application, stage deploymentskyscannernetv1alpha1.ProgressiveRolloutStage) bool {
	// TODO: add logic
	return false
}

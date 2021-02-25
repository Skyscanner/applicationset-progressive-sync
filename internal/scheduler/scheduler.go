package scheduler

import (
	deploymentskyscannernetv1alpha1 "github.com/Skyscanner/argocd-progressive-rollout/api/v1alpha1"
	"github.com/Skyscanner/argocd-progressive-rollout/internal/utils"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Scheduler returns a list of apps to sync for a given stage
func Scheduler(apps []argov1alpha1.Application, stage deploymentskyscannernetv1alpha1.ProgressiveRolloutStage) []string {

	/*
	The Scheduler takes
	- a ProgressiveRolloutStage object
	- the Applications selected with the clusters selector

	The Scheduler splits the Applications in the following groups:
	- OutOfSync Applications: those are the Applications to update during the stage.
	- syncedInCurrentStage Applications: those are Application that synced during the current stage. Those Applications count against the number of clusters to update.
	- progressingApps: those are Applications that are still in progress updating. Those Applications count against the number of clusters to update in parallel.
	*/

	var scheduledApps []string

	oufOfSyncApps := utils.GetAppsBySyncStatusCode(apps, argov1alpha1.SyncStatusCodeOutOfSync)
	syncedInCurrentStage := utils.GetSyncedAppsByStage(apps, stage.Name)
	progressingApps := utils.GetAppsByHealthStatusCode(apps, health.HealthStatusProgressing)

	maxTargets, err := intstr.GetScaledValueFromIntOrPercent(&stage.MaxTargets, len(oufOfSyncApps), false)
	if err != nil {
		return scheduledApps
	}
	maxParallel, err := intstr.GetScaledValueFromIntOrPercent(&stage.MaxParallel, maxTargets, false)
	if err != nil {
		return scheduledApps
	}

	if (maxTargets - len(syncedInCurrentStage)) > 0 {
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

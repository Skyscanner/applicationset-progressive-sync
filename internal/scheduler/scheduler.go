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
		The Scheduler takes:
			- a ProgressiveRolloutStage object
			- the Applications selected with the clusters selector

		The Scheduler splits the Applications in the following groups:
			- OutOfSync Applications: those are the Applications to update during the stage.
			- syncedInCurrentStage Applications: those are Application that synced during the current stage. Those Applications count against the number of clusters to update.
			- progressingApps: those are Applications that are still in progress updating. Those Applications count against the number of clusters to update in parallel.

		Why does the Scheduler need an annotation?
		Consider the scenario where we have 5 Applications - 4 OutOfSync and 1 Synced - and a stage with maxTargets = 3.
		If we don't keep track on which stage the Application synced, we can't compute how many applications we have to update in the current stage.
		Without the annotation, it would be impossible for the scheduler to know if the Application synced at this stage - and so we have only 2 Applications left to sync.
	*/

	var scheduledApps []string
	outOfSyncApps := utils.GetAppsBySyncStatusCode(apps, argov1alpha1.SyncStatusCodeOutOfSync)
	// If there are no OutOfSync Applications, return
	if len(outOfSyncApps) == 0 {
		return scheduledApps
	}

	syncedInCurrentStage := utils.GetSyncedAppsByStage(apps, stage.Name)
	progressingApps := utils.GetAppsByHealthStatusCode(apps, health.HealthStatusProgressing)

	maxTargets, err := intstr.GetScaledValueFromIntOrPercent(&stage.MaxTargets, len(outOfSyncApps), false)
	if err != nil {
		return scheduledApps
	}
	maxParallel, err := intstr.GetScaledValueFromIntOrPercent(&stage.MaxParallel, maxTargets, false)
	if err != nil {
		return scheduledApps
	}

	// We want to target minimum one cluster
	if maxTargets == 0 {
		maxTargets = 1
	}
	if maxParallel == 0 {
		maxParallel = 1
	}

	// If we already synced the desired number of Applications, return
	if maxTargets == len(syncedInCurrentStage) {
		return scheduledApps
	}

	for i := 0; i < maxParallel-len(progressingApps); i++ {
		scheduledApps = append(scheduledApps, outOfSyncApps[i].Name)
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

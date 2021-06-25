package scheduler

import (
	syncv1alpha1 "github.com/Skyscanner/applicationset-progressive-sync/api/v1alpha1"
	"github.com/Skyscanner/applicationset-progressive-sync/internal/utils"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Scheduler returns a list of apps to sync for a given stage
func Scheduler(apps []argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) []argov1alpha1.Application {

	/*
		The Scheduler takes:
			- a ProgressiveSyncStage object
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

	var scheduledApps []argov1alpha1.Application
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

	// Validation should never allow the user to explicitly use zero values for maxTargets or maxParallel.
	// Due the rounding down when scaled, they might resolve to 0.
	// If one of them resolve to 0, we set it to 1.
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

	// Because of eventual consistency, there might be a time where
	// maxParallel-len(progressingApps) might actually be greater than len(outOfSyncApps)
	// causing the runtime to panic
	p := maxParallel - len(progressingApps)
	if p > len(outOfSyncApps) {
		p = len(outOfSyncApps)
	}
	for i := 0; i < p; i++ {
		scheduledApps = append(scheduledApps, outOfSyncApps[i])
	}
	return scheduledApps
}

// IsStageFailed returns true if at least one app is failed
func IsStageFailed(apps []argov1alpha1.Application) bool {
	// An app is failed if:
	// - its Health Status Code is Degraded
	// - its Sync Status Code is Synced
	degradedApps := utils.GetAppsByHealthStatusCode(apps, health.HealthStatusDegraded)
	degradedSyncedApps := utils.GetAppsBySyncStatusCode(degradedApps, argov1alpha1.SyncStatusCodeSynced)
	return len(degradedSyncedApps) > 0
}

// IsStageInProgress returns true if at least one app is is in progress
func IsStageInProgress(apps []argov1alpha1.Application) bool {
	// An app is in progress if:
	// - its Health Status Code is Progressing
	progressingApps := utils.GetAppsByHealthStatusCode(apps, health.HealthStatusProgressing)
	return len(progressingApps) > 0
}

// IsStageComplete returns true if all applications are Synced and Healthy
func IsStageComplete(apps []argov1alpha1.Application, stage syncv1alpha1.ProgressiveSyncStage) bool {
	// An app is complete if:
	// - its Health Status Code is Healthy
	// - its Sync Status Code is Synced
	completeApps := utils.GetAppsByHealthStatusCode(apps, health.HealthStatusHealthy)
	completeSyncedApps := utils.GetAppsBySyncStatusCode(completeApps, argov1alpha1.SyncStatusCodeSynced)

	appsToCompleteStage := len(apps)
	if stage.MaxTargets.IntValue() < appsToCompleteStage {
		appsToCompleteStage = stage.MaxTargets.IntValue()
	}

	return len(completeSyncedApps) == appsToCompleteStage
}

package utils

import (
	"sort"

	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	corev1 "k8s.io/api/core/v1"
)

// IsArgoCDCluster returns true if one of the labels is the ArgoCD secret label with the secret type cluster as value
func IsArgoCDCluster(labels map[string]string) bool {
	val, ok := labels[ArgoCDSecretTypeLabel]
	return val == ArgoCDSecretTypeCluster && ok
}

// SortSecretsByName sort the SecretList in place by the secrets name
func SortSecretsByName(secrets *corev1.SecretList) {
	sort.SliceStable(secrets.Items, func(i, j int) bool { return secrets.Items[i].Name < secrets.Items[j].Name })
}

// SortAppsByName sort the Application slice in place by the app name
func SortAppsByName(apps *[]argov1alpha1.Application) {
	sort.SliceStable(*apps, func(i, j int) bool { return (*apps)[i].Name < (*apps)[j].Name })
}

// FilterAppsBySyncStatusCode returns the Applications matching the specified sync status code
func FilterAppsBySyncStatusCode(apps []argov1alpha1.Application, code argov1alpha1.SyncStatusCode) []argov1alpha1.Application {
	var result []argov1alpha1.Application

	for _, app := range apps {
		if app.Status.Sync.Status == code {
			result = append(result, app)
		}
	}

	return result
}

// GetAppsByHealthStatusCode returns the Applications matching the specified health status code
func GetAppsByHealthStatusCode(apps []argov1alpha1.Application, code health.HealthStatusCode) []argov1alpha1.Application {
	var result []argov1alpha1.Application

	for _, app := range apps {
		if app.Status.Health.Status == code {
			result = append(result, app)
		}
	}

	return result
}

// GetSyncedAppsByStage returns the Applications that synced during the given stage
func GetSyncedAppsByStage(apps []argov1alpha1.Application, name string) []argov1alpha1.Application {
	var result []argov1alpha1.Application

	for _, app := range apps {
		val, ok := app.Annotations[ProgressiveRolloutSyncedAtStageKey]
		if ok && val == name && app.Status.Sync.Status == argov1alpha1.SyncStatusCodeSynced && app.Status.Health.Status == health.HealthStatusHealthy {
			result = append(result, app)
		}
	}

	return result
}

// HasString returns true if a slice contains the given string
func HasString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// RemoveString returns a new slice without the given string
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

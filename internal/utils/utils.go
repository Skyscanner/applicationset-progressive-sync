package utils

import (
	"fmt"
	"sort"
	"strings"

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
func SortAppsByName(apps []argov1alpha1.Application) {
	sort.SliceStable(apps, func(i, j int) bool { return (apps)[i].Name < (apps)[j].Name })
}

// GetAppsBySyncStatusCode returns the Applications matching the specified sync status code
func GetAppsBySyncStatusCode(apps []argov1alpha1.Application, code argov1alpha1.SyncStatusCode) []argov1alpha1.Application {
	var result []argov1alpha1.Application

	for _, app := range apps {
		if app.Status.Sync.Status == code {
			result = append(result, app)
		}
	}

	return result
}

// GetAppsByAnnotation returns the Applications having the specified annotation
func GetAppsByAnnotation(apps []argov1alpha1.Application, annotation string, stageName string, syncedAtStage map[string]string, psName string) []argov1alpha1.Application {
	var result []argov1alpha1.Application

	for _, app := range apps {

		_ = app.Annotations[annotation]
		//
		//if ok && value == stageName {
		//	result = append(result, app)
		//}
		appKeyValue := psName + "/" + app.Name

		value, ok := syncedAtStage[appKeyValue]

		if ok && value == stageName {
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
func GetSyncedAppsByStage(apps []argov1alpha1.Application, name string, syncedAtStage map[string]string, psName string) []argov1alpha1.Application {
	var result []argov1alpha1.Application

	for _, app := range apps {
		//val, ok := app.Annotations[ProgressiveSyncSyncedAtStageKey]
		//if ok && val == name && app.Status.Sync.Status == argov1alpha1.SyncStatusCodeSynced {
		//	result = append(result, app)
		//}
		appKeyValue := psName + "/" + app.Name

		val, ok := syncedAtStage[appKeyValue]
		if ok && val == name && app.Status.Sync.Status == argov1alpha1.SyncStatusCodeSynced {
			result = append(result, app)
		}
	}

	return result
}

// GetAppsName returns a string containing a comma-separated list of names of the given apps
func GetAppsName(apps []argov1alpha1.Application) string {
	var names []string
	for _, a := range apps {
		names = append(names, a.GetName())
	}
	return fmt.Sprint(strings.Join(names, ", "))
}

// GetClustersName returns a string containing a comma-separated list of names of the given clusters
func GetClustersName(clusters []corev1.Secret) string {
	var names []string
	for _, c := range clusters {
		names = append(names, c.GetName())
	}
	return fmt.Sprint(strings.Join(names, ", "))
}

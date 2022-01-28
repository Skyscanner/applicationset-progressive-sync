package utils

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/Skyscanner/applicationset-progressive-sync/internal/consts"
	argov1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/davecgh/go-spew/spew"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

// IsArgoCDCluster returns true if one of the labels is the ArgoCD secret label with the secret type cluster as value
func IsArgoCDCluster(labels map[string]string) bool {
	val, ok := labels[consts.ArgoCDSecretTypeLabel]
	return val == consts.ArgoCDSecretTypeCluster && ok
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

// ComputeHash returns a hash value calculated from a spec using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func ComputeHash(spec interface{}) string {
	hasher := fnv.New32a()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hasher, "%#v", spec)

	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

// Min returns the minumum between to int
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// HaveSameRevision returns true if the given Applications have the same status.sync.revision
func HaveSameRevision(apps []argov1alpha1.Application) bool {
	set := make(map[string]struct{})

	for _, app := range apps {
		revision := app.Status.Sync.Revision
		if _, ok := set[revision]; !ok {
			set[revision] = struct{}{}
		}
	}

	return len(set) == 1
}

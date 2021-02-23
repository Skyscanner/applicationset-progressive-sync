package utils

import (
	corev1 "k8s.io/api/core/v1"
	"sort"
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

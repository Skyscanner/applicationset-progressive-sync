package utils

// IsArgoCDCluster returns true if one of the labels is the ArgoCD secret label with the secret type cluster as value
func IsArgoCDCluster(labels map[string]string) bool {
	val, ok := labels[ArgoCDSecretTypeLabel]
	return val == ArgoCDSecretTypeCluster && ok
}

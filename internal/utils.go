package internal

const (
	ArgoCDSecretTypeLabel   = "argocd.argoproj.io/secret-type"
	ArgoCDSecretTypeCluster = "cluster"
	AppSetKind              = "ApplicationSet"
	AppSetAPIGroup          = "argoproj.io/v1alpha1"
)

func IsArgoCDCluster(labels map[string]string) bool {
	val, ok := labels[ArgoCDSecretTypeLabel]
	if val == ArgoCDSecretTypeCluster && ok {
		return true
	} else {
		return false
	}
}

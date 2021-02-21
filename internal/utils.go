package internal

const (
	ArgoCDSecretTypeLabel   = "argocd.argoproj.io/secret-type"
	ArgoCDSecretTypeCluster = "cluster"
	AppSetKind              = "ApplicationSet"
	AppSetAPIGroup          = "argoproj.io/v1alpha1"
)

func IsArgoCDCluster(annotations map[string]string) bool {
	val, ok := annotations[ArgoCDSecretTypeLabel]
	if val == ArgoCDSecretTypeCluster && ok {
		return true
	} else {
		return false
	}
}

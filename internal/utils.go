package internal

const (
	ArgoCDSecretTypeLabel   = "argocd.argoproj.io/secret-type"
	ArgoCDSecretTypeCluster = "cluster"
)

func IsArgoCDCluster(annotations map[string]string) bool {
	val, ok := annotations[ArgoCDSecretTypeLabel]
	if val == ArgoCDSecretTypeCluster && ok {
		return true
	} else {
		return false
	}
}

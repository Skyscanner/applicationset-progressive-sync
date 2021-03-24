package utils

const (
	ArgoCDSecretTypeLabel              = "argocd.argoproj.io/secret-type"
	ArgoCDSecretTypeCluster            = "cluster"
	AppSetKind                         = "ApplicationSet"
	AppSetAPIGroup                     = "argoproj.io/v1alpha1"
	ArgoCDAuthTokenKey                 = "ARGOCD_AUTH_TOKEN"
	ArgoCDServerAddrKey                = "ARGOCD_SERVER_ADDR"
	ConfigDirectory                    = "/etc/prcconfig/"
	ProgressiveRolloutSyncedAtStageKey = "apr.skyscanner.net/syncedAtStage"
)

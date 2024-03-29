# applicationset-progressive-sync

![Version: 0.6.0-prealpha](https://img.shields.io/badge/Version-0.6.0--prealpha-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: sha-f70b92b](https://img.shields.io/badge/AppVersion-sha--f70b92b-informational?style=flat-square)

A Helm chart to install the applicationset-progressive-sync controller.

**Homepage:** <https://github.com/Skyscanner/applicationset-progressive-sync>

## Installing the Chart

To install the chart with the release name `my-release`:

```console
helm repo add applicationset-progressive-sync https://skyscanner.github.io/applicationset-progressive-sync/
helm upgrade -i my-release applicationset-progressive-sync/applicationset-progressive-sync --namespace argocd
```
## Values

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| args.leaderElect | bool | `false` | Enable leader election for controller manager. |
| args.metricsBindAddress | string | `":8080"` | The address the metric endpoint binds to. |
| config | object | `{"argoCDAuthToken":"example-token","argoCDInsecure":"true","argoCDPlaintext":"false","argoCDServerAddr":"argocd-server"}` | Config options |
| config.argoCDAuthToken | string | `"example-token"` | ArgoCD token |
| config.argoCDInsecure | string | `"true"` | Allow insecure connection with ArgoCD server |
| config.argoCDPlaintext | string | `"false"` | Allow http connection with ArgoCD server |
| config.argoCDServerAddr | string | `"argocd-server"` | ArgoCD server service address |
| configSecret | object | `{"annotations":{},"name":""}` | configSecret is a secret object which supplies tokens, configs, etc. |
| configSecret.name | string | `""` | If this value is not provided, a secret will be generated |
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/skyscanner/applicationset-progressive-sync"` |  |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion. |
| imagePullSecrets | list | `[]` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podSecurityContext | object | `{}` |  |
| rbac.pspEnabled | bool | `false` |  |
| replicaCount | int | `1` |  |
| resources | object | `{}` |  |
| securityContext | object | `{}` |  |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| tolerations | list | `[]` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.5.0](https://github.com/norwoodj/helm-docs/releases/v1.5.0)

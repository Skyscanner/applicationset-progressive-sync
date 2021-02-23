# argocd-progressive-rollout

`argocd-progressive-rollout` is a controller to allow a progressive rollout of ArgoCD Applications generated by an ApplicationSet.

## Motivation

[Argo ApplicationSet](https://github.com/argoproj-labs/applicationset) is being developed as the solution to replace the `app-of-apps` pattern.

While `ApplicationSet` is great to programmatically generate ArgoCD Applications, we will still need to solve _how_ to update the Applications.

If we enable the `auto-sync` policy, we will update all the generated Applications at once.

This might not be a problem if we have only one production cluster, but organizations with tens or hundreds of clusters need to avoid a global rollout. They need to release new versions of their application in a safer way.

The `argocd progressive rollout` controller allows operators and developers to decide _how_ they want to update their Applications.

## Example `spec`

```yaml
apiVersion: deployment.skyscanner.net/v1alpha1
kind: ProgressiveRollout
metadata:
  name: myprogressiverollout
  namespace: argoc
spec:
  # a reference to the target ApplicationSet
  sourceRef:
    apiVersion: argoproj.io/v1alpha1
    kind: ApplicationSet
    name: myappset
    # the rollout steps
  stages:
      # human friendly name
    - name: two clusters as canary in EMEA
      # how many targets to update in parallel
      # can be an integer or %. Default to 1
      maxParallel: 2
      # how many targets to update from the selector result
      # can be an integer or %. Default to 100%.
      maxTargets: 2
      # which targets to update
      targets:
        clusters:
          selector:
            matchLabels:
              area: emea
    - name: rollout to remaining clusters
      maxParallel: 25%
      maxTargets: 100%
      targets:
        clusters:
          selector: {}
```

## Status: `pre-alpha`

Expect a non-functional controller and breaking changes until Milestone 2 is completed.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md)

## Development

### Local development with Kubebuilder

1. Install `pre-commit`: see <https://pre-commit.com/#install>
1. Install `kubebuilder`: see <https://book.kubebuilder.io/quick-start.html#installation>
1. Install `ArgoCD Application` API pkg: see `hack/install-argocd-application.sh`

### Update ArgoCD Application API package

Because of [https://github.com/argoproj/argo-cd/issues/4055](https://github.com/argoproj/argo-cd/issues/4055) we can't just run `go get github.com/argoproj/argo-cd`.

Use `hack/install-argocd-application.sh` to install the correct version of the Application API.

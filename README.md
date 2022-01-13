# applicationset-progressive-sync

`applicationset-progressive-sync` is a controller to allow a progressive sync of ArgoCD Applications generated by an ApplicationSet.

## Motivation

[Argo ApplicationSet](https://github.com/argoproj-labs/applicationset) is being developed as the solution to replace the `app-of-apps` pattern.

While `ApplicationSet` is great to programmatically generate ArgoCD Applications, we will still need to solve _how_ to update the Applications.

If we enable the `auto-sync` policy, we will update all the generated Applications at once.

This might not be a problem if we have only one production cluster, but organizations with tens or hundreds of clusters need to avoid a global rollout. They need to release new versions of their application in a safer way.

The `applicationset-progressive-sync` controller allows operators and developers to decide _how_ they want to update their Applications.

## Example `spec`

```yaml
apiVersion: argoproj.skyscanner.net/v1alpha1
kind: ProgressiveSync
metadata:
  name: myprogressivesync
  namespace: argocd
spec:
  # a reference to the target ApplicationSet in the same namespace
  appSetRef:
    name: myappset
    # the rollout steps
  stages:
      # human friendly name
    - name: two clusters as canary in EMEA
      # how many targets to update in parallel
      # can be an integer or %.
      maxParallel: 2
      # how many targets to update from the selector result
      # can be an integer or %.
      maxTargets: 2
      # which targets to update
      targets:
        clusters:
          selector:
            matchLabels:
              area: emea
    - name: rollout to remaining clusters
      maxParallel: 2
      maxTargets: 4
      targets:
        clusters:
          selector: {}
```

## Status: `pre-alpha`

Expect a non-functional controller and breaking changes until Milestone 2 is completed.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md)

## Configuration

The controller connects to an Argo CD server and requires configuration to do so:

```console
ARGOCD_AUTH_TOKEN: <token of the Argo CD user>
ARGOCD_SERVER_ADDR: <address of the Argo CD server>
ARGOCD_INSECURE: <true/false>
```

The above configuration is loaded taking into account the following priority order:

1. Environment Variables.

```console
ARGOCD_AUTH_TOKEN=ey...
ARGOCD_SERVER_ADDR=argocd-server
ARGOCD_INSECURE=true
```

1. Files in the Config Directory (`/etc/applicationset-progressive-sync/`).

```console
/etc/
├── applicationset-progressive-sync/
│   ├── ARGOCD_AUTH_TOKEN  # file content: ey...
│   ├── ARGOCD_SERVER_ADDR # file content: argocd-server
│   ├── ARGOCD_INSECURE    # file content: true
```

If at least one of the options is missing, the controller will **fail** to start.

## Development

### Local development with Kubebuilder

To get the controller running against the configured Kubernetes cluster in ~/.kube/config, run:

```shell
make install
make run
```

Please remember the `ARGOCD_AUTH_TOKEN`, `ARGOCD_SERVER_ADDR` and `ARGOCD_INSECURE`  environment variables need to be present in order
to run against a Kubernetes cluster with Argo CD. If the cluster was configured using the `hack/setup-dev.sh` script,
these variables are part of the `.env.local` file.

### Deploying to a Kubernetes cluster

To deploy the controller to a Kubernetes cluster, run:

```shell
make install
make docker-build
make deploy
```

In order to do so, the target cluster needs to have a secret named `prc-config` containing the three necessary
variables: `ARGOCD_AUTH_TOKEN`, `ARGOCD_SERVER_ADDR` and `ARGOCD_INSECURE`. If using the dev environment
in the following section, this secret has already been created.

If using `kind` clusters, docker images need to be loaded manually using `kind load docker-image <image>:<version> --name <cluster-name>`.

#### Configuration Secret

Here's a sample secret of the necessary configuration:

```yaml
apiVersion: v1
data:
  ARGOCD_AUTH_TOKEN: ey...
  ARGOCD_INSECURE: true
  ARGOCD_SERVER_ADDR: argocd-server
kind: Secret
metadata:
  name: prc-config
  namespace: argocd
type: Opaque
```

### Setting up dev environment

To facilitate local debugging and testing against real clusters, you may run:

```shell
bash hack/install-dev-deps.sh
bash hack/setup-dev.sh [argocd-version] [appset-version]
make install
make deploy
```

this will install all the dependencies (`pre-commit`, `kubebuilder`, `argocd`, `kind`) and it will install the correct version of ArgoCD Application API package for you. If you omit `argocd-version` and/or `appset-version` it will default to the latest stable/tested versions of ArgoCD and Appset controller.

After running the script, you will have 3 kind clusters created locally:

- `kind-argocd-control-plane` - cluster hosting the argocd installation and the progressive sync operator. This cluster is also registered with Argo so that we can simulate using the same process for deploying to control cluster as well
- `kind-prc-cluster-1` and `kind-prc-cluster-2` - are the target clusters for deploying the apps to.

This gives us a total of 3 clusters allowing us to play with multiple stages of deploying. It will also log you in argocd cli. You can find additional login details in `.env.local` file that will be generated for your convenience.

#### Regenerating your access

In case that your access to the local argocd has become broken, you can regenerate it by running

```shell
bash hack/login-argocd-local.sh
```

This will create a socat link in kind docker network allowing you to access argocd server UI through your localhost.
The exact port will be outputted after the command has been run. Running this command will also update the values in `.env.local`.

#### Registering additional clusters

If you want to create additional clusters, you can do so by running:

```shell
bash hack/add-cluster <cluster-name> <recreate> <labels>
```

This will spin up another kind cluster and register it against ArgoCD running in `kind-argocd-control-plane`

#### Deploying local test resources

You can deploy a test appset and a progressive sync object to your kind environment via:

```shell
bash hack/redeploy-dev-resources.sh
```

Feel free to extend the cluster generation section of the appset spec if you want to deploy it clusters that you have manually created.

### Debugging

```shell
make debug
```

Invoking the command above should spin up a Delve debugger server in headless mode. You can then use your IDE specific functionality or the delve client itself to attach to the remote process and debug it.

**NOTE**: On MacOSX, delve is currently unkillable in headless mode with `^C` or any other control signals that can be sent from the same terminal session. Instead, you'd need to run

``` shell
bash ./hack/kill-debug.sh
```

or

``` shell
make debug
```

from another terminal session to kill the debugger.

### Debugging tests

Delve can be used to debug tests as well. See `Test` launch configuration in `.vscode/launch.json`. Something similar should be achievable in your IDE of choice as well.

#### Update ArgoCD Application API package

Because of [https://github.com/argoproj/argo-cd/issues/4055](https://github.com/argoproj/argo-cd/issues/4055) we can't just run `go get github.com/argoproj/argo-cd`.

Use `hack/install-argocd-application.sh` to install the correct version of the Application API.

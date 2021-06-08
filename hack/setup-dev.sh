#!/bin/bash
set -e

argocd_version=${1:-"v1.7.14"}
appset_version=${2:-"v0.1.0"}

root=$(dirname "${BASH_SOURCE[0]}")

bash -e "$root"/install-dev-deps.sh true
# shellcheck source=hack/dev-functions.sh
source "$root"/dev-functions.sh

# Create the control cluster
kind delete cluster --name argocd-control-plane
kind create cluster --name argocd-control-plane
kubectl create namespace argocd

# Setup argocd
kubectl apply -n argocd -f "https://raw.githubusercontent.com/argoproj/argo-cd/$argocd_version/manifests/install.yaml"
kubectl apply -n argocd -f "https://raw.githubusercontent.com/argoproj-labs/applicationset/$appset_version/manifests/install.yaml"

# Login as admin and change def pass

# It can take a few secs for kubernetes to create the object, if we try to watch when the object doesn't exist yet
# the watch will fail, so just sleep for a bit.
sleep 10

echo "Waiting for ArgoCD server to become ready. This can take up to 5 minutes.."
kubectl wait --for=condition=ready --timeout=300s pod -l app.kubernetes.io/name=argocd-server -n argocd

argoserver=$(kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-server -o name | cut -d'/' -f 2)
argocdlogin_initial="argocd login --insecure --username admin --password $argoserver argocd-server.argocd.svc.cluster.local:443"
retry_argocd_exec "$argocdlogin_initial && argocd account update-password --account admin --current-password $argoserver --new-password admin"
argocdlogin="argocd login --insecure --username admin --password admin argocd-server.argocd.svc.cluster.local:443"

# # Create a new user
kubectl apply -f "$root"/dev/users.yml
kubectl apply -f "$root"/dev/secrets.yml
# and set its password
retry_argocd_exec "$argocdlogin && argocd account update-password --account prc --current-password admin --new-password prc" || echo "Success"

# Setup permissions for the new user
kubectl apply -f "$root"/dev/rbac.yml
# Register in-cluster in argo secrets
kubectl apply -f "$root"/dev/control-plane.yml

label_argocd_cluster "cluster-kubernetes.default" "region=eu-central-1"

# Create additional clusters to server as deployment targets
register_argocd_cluster "prc-cluster-1" true
register_argocd_cluster "prc-cluster-2" true

label_argocd_cluster "prc-cluster-1" "region=eu-west-1"
label_argocd_cluster "prc-cluster-2" "region=ap-northeast-1"

local_address=$(local_argocd_login)

# retry_argocd_exec can race with outputting to stdout with the rest of this script
# so sleep for a bit to make sure that the token and server ip output doesn't get mangled
sleep 5

echo "All done"
echo "ArgoCD server is available at: $local_address"
echo "You can find the password and the token stored in .env.local file for your convenience."

open "$local_address"

# TODO: Deploy prog rollout controller to control cluster
# TODO: Deploy a sample ProgRollout CRD to control cluster

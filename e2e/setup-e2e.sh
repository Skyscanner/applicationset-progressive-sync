#!/usr/bin/env bash

set -euo pipefail

log() {
    echo -e "\n\033[1m$1\033[0m"
}

CLUSTERS=(
    control
    account1-eu-west-1a-1
    account1-eu-west-1b-1
)
CONTROL="${CLUSTERS[0]}"
WORKLOAD=("${CLUSTERS[@]:1}")
LOAD_BALANCER_PORT=8084
ARGOCD_VERSION="v2.2.3"
PORT=6440
ORG_DOMAIN="${ORG_DOMAIN:-progressivesync.skyscanner.io}"

log "Creating control cluster"

if k3d cluster get "${CONTROL}" >/dev/null 2>&1 ; then
    echo "Already exists: ${CONTROL}" >&2
else
    k3d cluster create "${CONTROL}" \
        --api-port="$((PORT++))" \
        -p "${LOAD_BALANCER_PORT}:80@loadbalancer" \
        --network=multicluster \
        --k3s-arg="--cluster-domain=${CONTROL}.${ORG_DOMAIN}@server:0" \
        --wait
fi

log "Creating workload clusters"

for cluster in "${WORKLOAD[@]}" ; do
    if k3d cluster get "${cluster}" >/dev/null 2>&1 ; then
        echo "Already exists: ${cluster}" >&2
    else
        k3d cluster create "${cluster}" \
            --api-port="$((PORT++))" \
            --network=multicluster \
            --k3s-arg="--cluster-domain=${cluster}.${ORG_DOMAIN}@server:0" \
            --wait
    fi
done

log "Installing ArgoCD and ApplicationSet controller"

# Install ArgoCD on control cluster
kubectl --context "k3d-${CONTROL}" create namespace argocd || true
kubectl --context "k3d-${CONTROL}" -n argocd apply -f https://raw.githubusercontent.com/argoproj/argo-cd/"${ARGOCD_VERSION}"/manifests/install.yaml

log "Installing ingress"

# Patch ArgoCD server to run locally
kubectl --context "k3d-${CONTROL}" -n argocd patch configmaps argocd-cmd-params-cm --type merge -p '{"data":{"server.insecure":"false"}}'

# Restart ArgoCD server
kubectl --context "k3d-${CONTROL}" -n argocd rollout restart deployment argocd-server

# Add ingress object
# kubectl --context "k3d-${CONTROL}" -n argocd apply -f "$(dirname "$0")/manifests/argocd-ingress.yaml"

# password=$(kubectl --context "k3d-${CONTROL}" -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
# echo "ArgoCD password: ${password}"

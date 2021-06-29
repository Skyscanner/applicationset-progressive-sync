#!/bin/bash
set -e

root=$(dirname "${BASH_SOURCE[0]}")

prevcontext=$(kubectl config current-context)
kubectl config use-context kind-argocd-control-plane

# Remove finalizer to allow deletion to complete
kubectl patch progressivesync -n argocd goinfra -p '{"metadata":{"finalizers":null}}' --type=merge || echo "Not found. Continuing..."

kubectl delete progressivesync -n argocd goinfra || echo "Not found. Continuing..."
kubectl apply -f "$root"/dev/test-progsync.yml

kubectl config use-context "$prevcontext"

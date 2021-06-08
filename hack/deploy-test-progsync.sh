#!/bin/bash
set -e

root=$(dirname "${BASH_SOURCE[0]}")

prevcontext=$(kubectl config current-context)
kubectl config use-context kind-argocd-control-plane

# Remove finalizer to allow deletion to complete
(kubectl get progressivesync -n argocd goinfra -o=json | jq '.metadata.finalizers = null' | kubectl apply -f -) || echo "Continuing.."
kubectl delete progressivesync -n argocd goinfra || echo "Continuing.."
kubectl apply -f "$root"/dev/test-progsync.yml

kubectl config use-context "$prevcontext"

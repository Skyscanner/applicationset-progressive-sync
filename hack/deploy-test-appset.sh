#!/bin/bash
set -e

root=$(dirname "${BASH_SOURCE[0]}")

prevcontext=$(kubectl config current-context)
kubectl config use-context kind-argocd-control-plane

kubectl delete --all applications -n argocd
kubectl delete applicationset -n argocd appset-goinfra || echo "Not found"
kubectl apply -f "$root"/dev/test-appset.yml

# TODO: Make this generate argo apps in all created clusters
kubectl create ns infrabin || echo "infrabin already exists"

kubectl config use-context kind-prc-cluster-1
kubectl create ns infrabin || echo "infrabin already exists"

kubectl config use-context kind-prc-cluster-2
kubectl create ns infrabin || echo "infrabin already exists"

kubectl config use-context "$prevcontext"

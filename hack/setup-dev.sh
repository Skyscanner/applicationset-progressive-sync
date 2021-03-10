#!/bin/bash
set -e

root=$(dirname "${BASH_SOURCE[0]}")

bash -e $root/install-dev-deps.sh
source $root/dev-functions.sh

# Create the control cluster
kind delete cluster --name argocd-control-plane
kind create cluster --name argocd-control-plane
kubectl create namespace argocd

# Setup argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/applicationset/v0.1.0/manifests/install.yaml

# Login as admin and change def pass
sleep 10
kubectl wait --for=condition=ready --timeout=300s pod -l app.kubernetes.io/name=argocd-server -n argocd
argoserver=$(kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-server -o name | cut -d'/' -f 2)
argocdlogin_initial="argocd login --insecure --username admin --password $argoserver argocd-server.argocd.svc.cluster.local:443"
retry_argocd_exec "$argocdlogin_initial && argocd account update-password --account admin --current-password $argoserver --new-password admin"
argocdlogin="argocd login --insecure --username admin --password admin argocd-server.argocd.svc.cluster.local:443"

# # Create a new user
kubectl apply -f $root/dev/users.yml 
kubectl apply -f $root/dev/secrets.yml
# and set its password
retry_argocd_exec "$argocdlogin && argocd account update-password --account prc --current-password admin --new-password prc" || echo "Success"

# Setup permissions for the new user
kubectl apply -f $root/dev/perms.yml
# Register in-cluster in argo secrets
kubectl apply -f $root/dev/control-plane.yml

# Create additional clusters to server as deployment targets
register_argocd_cluster "prc-cluster-1" true
register_argocd_cluster "prc-cluster-2" true

# Generate token for the new user
token=$(retry_argocd_exec "$argocdlogin >/dev/null && argocd account generate-token --account prc")
serverip=$(kubectl get service -n argocd argocd-server -o=jsonpath='{.spec.clusterIP}')
sleep 5
echo "All done"
echo "Token:$token"
echo "Argo server ip:$serverip"
echo "Take note of token and server ip above. They will be required for setting up the PRC controller so that it can trigger syncs"

kubectl create secret generic -n argocd prc-controller-secret --from-literal="token=$token" --from-literal="serverip=$serverip"

# TODO: Deploy prog rollout controller to control cluster
# TODO: Deploy a sample ProgRollout CRD to control cluster



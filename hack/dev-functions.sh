
#!/bin/bash
set -e

function echoerr() { echo "$@" 1>&2; }

function retry_argocd_exec() {
    argoserver=$(kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-server -o name | cut -d'/' -f 2)
    command=$1
    max_retry=${2:-5}
    sleep_time=${3:-5}
    counter=0

    until kubectl exec -n argocd -it $argoserver -- bash -c "$command"
    do
    sleep $sleep_time
    [[ counter -eq $max_retry ]] && echoerr "Failed!" && exit 1
    echoerr "Trying again. Try #$counter"
    ((counter++))
    done
}

function register_argocd_cluster() {

    clustername=$1
    recreate=${2:-false}

    if [ "$recreate" = true ] ; then
        kind delete cluster --name $clustername
    fi

    # TODO: move these into a function
    argoserver=$(kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-server -o name | cut -d'/' -f 2)
    argocdlogin="argocd login --insecure --username admin --password admin argocd-server.argocd.svc.cluster.local:443"
    prevcontext=$(kubectl config current-context)

    kind create cluster --name $clustername

    kind get kubeconfig --name $clustername --internal > /tmp/$clustername.yml
    kubectl config use-context kind-argocd-control-plane
    kubectl cp /tmp/$clustername.yml argocd/$argoserver:/tmp/$clustername.yml
    retry_argocd_exec "$argocdlogin && argocd cluster add kind-$clustername --kubeconfig /tmp/$clustername.yml --upsert" || echo "Success after retrying"
    kubectl config use-context $prevcontext
}
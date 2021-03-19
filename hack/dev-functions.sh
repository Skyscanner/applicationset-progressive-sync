#!/bin/bash
set -eu

function err() {
	echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')]: $*" >&2
}

function retry_argocd_exec() {
	argoserver=$(kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-server -o jsonpath='{.items..metadata.name}')
	command=$1
	max_retry=${2:-5}
	sleep_time=${3:-5}
	counter=0

	until kubectl exec -n argocd -it "$argoserver" -- bash -c "$command"; do
		sleep "$sleep_time"
		[[ counter -eq $max_retry ]] && err "Failed!" && exit 1
		err "Trying again. Try #$counter"
		((counter++))
	done
}

function register_argocd_cluster() {

	clustername=$1
	recreate=${2:-false}

	if [[ "$recreate" == true ]]; then
		kind delete cluster --name "$clustername"
	fi

	# TODO: move these into a function
	argoserver=$(kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-server -o jsonpath='{.items..metadata.name}')
	argocdlogin="argocd login --insecure --username admin --password admin argocd-server.argocd.svc.cluster.local:443"
	prevcontext=$(kubectl config current-context)

	kind create cluster --name "$clustername"

	kind get kubeconfig --name "$clustername" --internal >/tmp/"$clustername".yml
	kubectl config use-context kind-argocd-control-plane
	kubectl cp /tmp/"$clustername".yml argocd/"$argoserver":/tmp/"$clustername".yml
	retry_argocd_exec "$argocdlogin && argocd cluster add kind-$clustername --kubeconfig /tmp/$clustername.yml --upsert" || echo "Success after retrying"
	kubectl config use-context "$prevcontext"
}

function local_argocd_login() {

	prevcontext=$(kubectl config current-context)
	kubectl config use-context kind-argocd-control-plane >/dev/null

	kubectl patch svc argocd-server -n argocd -p '{"spec": {"type": "NodePort"}}' >/dev/null
	controlplane_container=$(docker ps -aqf "name=argocd-control-plane")
	socat_container=$(docker ps -aqf "name=argocd-server-socat-proxy")
	argoserver_nodeport=$(kubectl get service -n argocd argocd-server -o=jsonpath='{.spec.ports[?(@.name=="https")].nodePort}')
	argocdlogin="argocd login --insecure --username admin --password admin argocd-server.argocd.svc.cluster.local:443"

	# Generate token for prc user
	token=$(retry_argocd_exec "$argocdlogin >/dev/null && argocd account generate-token --account prc")
	serverip=$(kubectl get service -n argocd argocd-server -o=jsonpath='{.spec.clusterIP}')

	kubectl config use-context "$prevcontext" >/dev/null

	docker stop "$socat_container" >/dev/null || err "No proxy container running"
	docker rm "$socat_container" >/dev/null || err "No proxy container running"

	# Kind isn't kind when it comes to exposing additional nodeports
	# since they also need to be mapped to docker
	# However, we can run socat to proxy a tcp socket without having to meddle with
	# the kind controlplane container. See: https://github.com/kubernetes-sigs/kind/issues/99#issuecomment-456184883
	docker run \
		--name argocd-server-socat-proxy \
		--detach \
		--restart always \
		--publish "$argoserver_nodeport":"$argoserver_nodeport" \
		--link "$controlplane_container":target \
		--network kind \
		alpine/socat \
		tcp-listen:"$argoserver_nodeport",fork,reuseaddr tcp-connect:target:"$argoserver_nodeport" >/dev/null

	argocd login --insecure --username prc --password prc localhost:"$argoserver_nodeport" >/dev/null

	{
		echo "ARGOCD_TOKEN=$token"
		echo "ARGOCD_USERNAME=prc"
		echo "ARGOCD_PASSWORD=prc"
		echo "ARGOCD_INCLUSTER_ADDRESS=$serverip"
		echo "ARGOCD_LOCAL_ADDRESS=https://localhost:$argoserver_nodeport"
	} >.env.local

	if [ ! -d .prcconfig ]; then
		mkdir -p .prcconfig
	fi

	echo -n "$token" | tr -d '\r' >.prcconfig/argocd-auth-token
	echo -n "localhost:$argoserver_nodeport" >.prcconfig/argocd-server-addr

	# Create a secret storing the token and in-cluster ip
	kubectl delete secret generic -n argocd prc-controller-secret >/dev/null || err "Secret not found. Creating.."
	kubectl create secret generic -n argocd prc-controller-secret --from-literal="token=$token" --from-literal="serverip=$serverip" >/dev/null

	echo "https://localhost:$argoserver_nodeport"
}

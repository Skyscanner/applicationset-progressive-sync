#!/bin/bash
set -euo pipefail

os=$(go env GOOS)
arch=$(go env GOARCH)
gopath=$(go env GOPATH)
root=$(dirname "${BASH_SOURCE[0]}")

check_only=${1:-false}
kubebuilder_version=${2:-"2.3.1"}
kind_version=${3:-"v0.10.0"}

root=$(dirname "${BASH_SOURCE[0]}")
# shellcheck source=hack/dev-functions.sh
source "$root"/dev-functions.sh

if ! [ -x "$(command -v pre-commit)" ]; then
	[ "$check_only" == true ] && err "pre-commit is not installed. Run bash hack/install-dev-deps.sh to install." && exit 1
	pip install --index https://pypi.python.org/simple pre-commit
fi

if [[ ! -d "/usr/local/kubebuilder" ]]; then
	[ "$check_only" == true ] && err "kubebuilder is not installed. Run bash hack/install-dev-deps.sh to install." && exit 1

	# download kubebuilder and extract it to tmp
	curl -L "https://go.kubebuilder.io/dl/${kubebuilder_version}/${os}/${arch}" | tar -xz -C /tmp/

	# move to a long-term location and put it on your path
	# (you'll need to set the KUBEBUILDER_ASSETS env var if you put it somewhere else)
	sudo mv -f "/tmp/kubebuilder_${kubebuilder_version}_${os}_${arch}" /usr/local/kubebuilder
	export PATH=$PATH:/usr/local/kubebuilder/bin
fi

if [[ ! -f "$root/.hack.argocd.installed" ]]; then
	[ "$check_only" == true ] && err "argocd application hack is not installed. Run bash hack/install-dev-deps.sh to install." && exit 1

	# Because of https://github.com/argoproj/argo-cd/issues/4055) we can't just run `go get github.com/argoproj/argo-cd`.
	bash -xe "$root"/install-argocd-application.sh
fi

# Install kind
if [[ ! -f "$gopath/bin/kind" ]]; then
	[ "$check_only" == true ] && err "kind is not installed. Run bash hack/install-dev-deps.sh to install." && exit 1

	curl -Lo ./kind "https://kind.sigs.k8s.io/dl/${kind_version}/kind-$os-$arch"
	chmod +x ./kind
	mv ./kind "$gopath/bin/kind"
fi

# Install argocd cli
if ! [ -x "$(command -v argocd)" ]; then
	[ "$check_only" == true ] && err "argocd is not installed. Run bash hack/install-dev-deps.sh to install." && exit 1

	argocd_version=$(curl --silent "https://api.github.com/repos/argoproj/argo-cd/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
	curl -sSL -o /usr/local/bin/argocd "https://github.com/argoproj/argo-cd/releases/download/$argocd_version/argocd-linux-amd64"
	chmod +x /usr/local/bin/argocd
fi

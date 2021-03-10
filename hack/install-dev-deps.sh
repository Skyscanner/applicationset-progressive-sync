#!/bin/bash
set -e

os=$(go env GOOS)
arch=$(go env GOARCH)
gopath=$(go env GOPATH)

if ! [ -x "$(command -v pre-commit)" ]
then
    pip install --index https://pypi.python.org/simple pre-commit
fi

go get github.com/tsg/gotpl

if [ ! -d "/usr/local/kubebuilder" ] 
then
    # download kubebuilder and extract it to tmp
    curl -L https://go.kubebuilder.io/dl/2.3.1/${os}/${arch} | tar -xz -C /tmp/

    # move to a long-term location and put it on your path
    # (you'll need to set the KUBEBUILDER_ASSETS env var if you put it somewhere else)
    sudo mv -f /tmp/kubebuilder_2.3.1_${os}_${arch} /usr/local/kubebuilder
    export PATH=$PATH:/usr/local/kubebuilder/bin
fi

# Because of https://github.com/argoproj/argo-cd/issues/4055) we can't just run `go get github.com/argoproj/argo-cd`.
bash -xe $root/install-argocd-application.sh

# Install kind
if [ ! -f "$gopath/bin/kind" ] 
then
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.10.0/kind-$os-$arch
    chmod +x ./kind
    mv ./kind $gopath/bin/kind
fi

# Install argocd cli
if ! [ -x "$(command -v argocd)" ]
then
    argocd_version=$(curl --silent "https://api.github.com/repos/argoproj/argo-cd/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    curl -sSL -o /usr/local/bin/argocd https://github.com/argoproj/argo-cd/releases/download/$argocd_version/argocd-linux-amd64
    chmod +x /usr/local/bin/argocd
fi
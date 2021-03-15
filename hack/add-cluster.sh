#!/bin/bash
set -eu

root=$(dirname "${BASH_SOURCE[0]}")
bash "$root"/install-dev-deps.sh
# shellcheck source=hack/dev-functions.sh
source "$root"/dev-functions.sh

clustername=$1
recreate=$2

if [[ -z "$clustername" ]]; then
	echo "Please provide a cluster name"
	exit 1
fi

register_argocd_cluster "$clustername" "$recreate"

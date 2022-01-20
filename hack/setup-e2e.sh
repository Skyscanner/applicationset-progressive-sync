#!/usr/bin/env bash

set -euo pipefail

source common.sh

for cluster in "${CLUSTERS[@]}" ; do
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
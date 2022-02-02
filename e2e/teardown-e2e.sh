#!/usr/bin/env bash

CLUSTERS=(
    control
    account1-eu-west-1a-1
    account1-eu-west-1b-1
)

for cluster in "${CLUSTERS[@]}" ; do
    if ! k3d cluster get "${cluster}" >/dev/null 2>&1 ; then
        echo "Already deleted: ${cluster}" >&2
    else
        k3d cluster delete "${cluster}"
    fi
done
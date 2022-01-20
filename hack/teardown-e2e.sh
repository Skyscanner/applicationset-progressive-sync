#!/usr/bin/env bash

source common.sh

for cluster in "${CLUSTERS[@]}" ; do
    if ! k3d cluster get "${cluster}" >/dev/null 2>&1 ; then
        echo "Already deleted: ${cluster}" >&2
    else
        k3d cluster delete "${cluster}"
    fi
done
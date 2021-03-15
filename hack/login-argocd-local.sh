#!/bin/bash
set -eu

root=$(dirname "${BASH_SOURCE[0]}")
# shellcheck source=hack/dev-functions.sh
source "$root"/dev-functions.sh

local_argocd_login

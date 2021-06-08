#!/bin/bash
set -e

root=$(dirname "${BASH_SOURCE[0]}")

# shellcheck source=.env.local
source .env.local


bash "$root/redeploy-dev-resources.sh"
dlv --listen=:2345 --headless=true --api-version=2 debug main.go --  --zap-devel=true
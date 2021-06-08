#!/bin/bash
set -e

root=$(dirname "${BASH_SOURCE[0]}")

bash "$root/deploy-test-appset.sh"
bash "$root/deploy-test-progsync.sh"

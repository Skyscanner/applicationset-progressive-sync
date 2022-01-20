#!/usr/bin/env bash

CLUSTERS=(
    control
    account1-eu-west-1a-1
    account1-eu-west-1b-1
)
PORT=6440
ORG_DOMAIN="${ORG_DOMAIN:-progressivesync.skyscanner.io}"
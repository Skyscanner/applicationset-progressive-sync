#!/bin/bash
set -e

# On mac delve refuses to die, so call this script to kill it and the actual go process it spawned
pgrep __debug_bin | xargs kill -SIGINT
pgrep dlv | xargs kill -SIGKILL

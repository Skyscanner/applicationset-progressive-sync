#!/usr/bin/env bash 
printenv

if [[ "${DEBUG}" == "TRUE" ]]; then
    # Delve in headless mode ignores SIGINT (ctrl+c), so the only way to kill it is
    # to invoke kill DELVE_PID from another process. This has an interesting side-effect, if the applicatio being debugged dies
    # or your debugger loses connection, delve process wil be left hanging without being able to reconnect
    # You will have to manually kill it or recycle the pod if this is deployed to k8s..
    echo "Starting manager in debug mode"
    dlv --listen=:2345 --headless=true --api-version=2 exec ./manager-debug -- "$@"
else
    echo "Starting manager in release mode"
    ./manager "$@"
fi
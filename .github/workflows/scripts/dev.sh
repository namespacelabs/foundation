#!/bin/sh
set -e

NS=$1

tmux new-session -d -s NsDevSession "$NS dev  --buildkit_import_cache=type=gha --buildkit_export_cache=type=gha,mode=max --golang_use_buildkit=true --build_in_nscloud internal/testdata/server/gogrpc"

COUNTER=0
while true ; do
    echo waiting for deployment to be created

    # Capture exit code since this command will fail if the deployment does not exist yet
    set +e
    $NS kubectl -- rollout status --watch --timeout=90s deployment/gogrpcserver-7hzne001dff2rpdxav703bwqwc 2> response.txt

    EXIT_CODE=$?
    set -e

    # Don't check what is missing, as we first lack "namespaces" then "deployments.apps"
    if ! cat response.txt | grep -q 'NotFound'; then
        cat response.txt

        if [[ $EXIT_CODE -gt 0 ]]; then
            exit $EXIT_CODE
        fi

        echo rollout complete
        break
    fi

    COUNTER=$((COUNTER+1))
    if [[ $COUNTER -ge 10 ]]; then
        echo giving up
        exit 1
    fi

    sleep 10
done

$NS kubectl -- get all

tmux send-keys -t NsDevSession -l q
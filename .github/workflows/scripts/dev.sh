#!/bin/sh
set -e

NS=$1

tmux new-session -d -s NsDevSession "$NS dev  --buildkit_import_cache=type=gha --buildkit_export_cache=type=gha,mode=max --golang_use_buildkit=false internal/testdata/server/gogrpc"

COUNTER=0
while true ; do
    echo waiting for deployment to be created

    set +e
    $NS kubectl -- rollout status --watch --timeout=90s deployment/gogrpcserver-7hzne001dff2rpdxav703bwqwc 2> response.txt
    set -e

    # Don't check what is missing, as we first lack "namespaces" then "deployments.apps"
    if ! cat response.txt | grep -q 'NotFound'; then
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

cat response.txt

$NS kubectl -- get all

tmux send-keys -t NsDevSession -l q
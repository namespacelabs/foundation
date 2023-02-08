#!/bin/sh
set -e

tmux new-session -d -s NsDevSession '/tmp/ns dev --buildkit_import_cache=type=gha --buildkit_export_cache=type=gha,mode=max --golang_use_buildkit=false internal/testdata/server/gogrpc'

COUNTER=0
while true ; do
    echo checking roolout status

    /tmp/ns kubectl -- rollout status --watch --timeout=90s deployment/gogrpcserver-7hzne001dff2rpdxav703bwqwc > response.txt

    # Don't check what is missing, as we first lack "namespaces" then "deployments.apps"
    if ! cat response.txt | grep -q 'NotFound'; then
        echo rollout complete
        break
    fi

    counter=$((counter+1))
    if [[ $COUNTER -ge 10 ]]; then
        echo give up
        break
    fi

    sleep 5
done

cat response.txt

/tmp/ns kubectl -- get all -A

tmux send-keys -t NsDevSession -l q
tmux attach-session -t NsDevSession
#!/bin/sh

tmux new-session -d -s NsDevSession '/tmp/ns dev --buildkit_import_cache=type=gha --buildkit_export_cache=type=gha,mode=max --golang_use_buildkit=false --naming_no_tls=true internal/testdata/server/gogrpc'

COUNTER=0
while true ; do
    RESPONSE=`/tmp/ns kubectl -- rollout status --watch --timeout=90s deployment/gogrpcserver-7hzne001dff2rpdxav703bwqwc`

    if [[ $RESPONSE != *"Error from server (NotFound)"* ]]; then
        # Don't check what is missing, as we first lack "namespaces" then "deployments.apps"
        break
    fi

    let COUNTER++

    if [[ $COUNTER -ge 10 ]]; then
        break
    fi

    sleep 5
done

echo $RESPONSE

/tmp/ns kubectl -- get all -A

tmux send-keys -t NsDevSession -l q
tmux attach-session -t NsDevSession
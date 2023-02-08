#!/bin/sh

tmux new-session -d -s NsDevSession '/tmp/ns dev --buildkit_import_cache=type=gha --buildkit_export_cache=type=gha,mode=max --golang_use_buildkit=false --naming_no_tls=true internal/testdata/integrations/dockerfile/complex'

while true ; do
    RESPONSE=`/tmp/ns kubectl -- rollout status --watch --timeout=90s deployment/mycomplexserver-mdpnh1j3smf8t83e`

    if [[ $RESPONSE != *"Error from server (NotFound): deployments.apps"* ]]; then
        break
    fi

    sleep 5
done

echo $RESPONSE

/tmp/ns kubectl get all

tmux send-keys -t NsDevSession -l q
tmux attach-session -t NsDevSession
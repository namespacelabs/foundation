#!/bin/sh

tmux new-session -d -s NsDevSession '/tmp/ns dev --buildkit_import_cache=type=gha --buildkit_export_cache=type=gha,mode=max --golang_use_buildkit=false --naming_no_tls=true internal/testdata/integrations/dockerfile/complex'

tmux ls

sleep 10
/tmp/ns kubectl get all
/tmp/ns kubectl -- rollout status --watch --timeout=90s deployment/mycomplexserver-mdpnh1j3smf8t83e

tmux send-keys -t NsDevSession -l q
tmux attach-session -t NsDevSession
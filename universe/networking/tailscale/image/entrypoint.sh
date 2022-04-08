#!/bin/bash

SOCKET="/tmp/tailscaled.sock"

tailscaled --tun=userspace-networking --socket=${SOCKET} --state=mem: &
PID=$!

TSKEY_PATH=`jq -r '.configure[] | select(.packageName=="namespacelabs.dev/foundation/universe/networking/tailscale").secret[] |  select(.name=="tailscale-auth-key").fromPath' /secrets/server/map.json`

tailscale --socket=${SOCKET} up --accept-dns=false --authkey=file:${TSKEY_PATH}

wait ${PID}
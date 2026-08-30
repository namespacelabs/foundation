#!/bin/bash

SOCKET="/tmp/tailscaled.sock"

tailscaled --tun=userspace-networking --socket=${SOCKET} --state=mem: &
PID=$!

tailscale --socket=${SOCKET} up --accept-dns=false --hostname=${FN_SERVER_NAME}-${FN_KUBERNETES_NAMESPACE}-${FN_SERVER_ID} --authkey=${TAILSCALE_AUTH_KEY}

wait ${PID}
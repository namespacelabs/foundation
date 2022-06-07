#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# Envoy start-up command
ENVOY=${ENVOY:-/usr/local/bin/envoy}

# Start envoy: important to keep drain time short
(${ENVOY} -c /bootstrap-xds.yaml --drain-time-s 1 -l debug)&
ENVOY_PID=$!

function cleanup() {
  kill ${ENVOY_PID}
}
trap cleanup EXIT

wait ${ENVOY_PID}
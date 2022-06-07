#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# Envoy start-up command
ENVOY=${ENVOY:-/usr/local/bin/envoy}

(${ENVOY} -c /bootstrap-xds.yaml)&
ENVOY_PID=$!

function cleanup() {
  kill ${ENVOY_PID}
}
trap cleanup EXIT

wait ${ENVOY_PID}
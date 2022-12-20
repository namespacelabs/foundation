#!/bin/bash

echo "injected endpoint is $ENDPOINT"

RESPONSE=`curl -s $ENDPOINT/mypath`

if [[ "$RESPONSE" != *"Hello from complex docker"* ]]; then
    echo "Unexpected response: $RESPONSE"
    exit 1
fi

# Testing Namespace itself: verifying that `fromServiceIngress` works.
# In tests the result is the in-cluster address.
if [[ "$INGRESS" != "http://webapi-mdpnh1j3smf8t83e:4000" ]]; then
    echo "Unexpected ingress: $INGRESS"
    exit 1
fi

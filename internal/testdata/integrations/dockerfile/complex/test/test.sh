#!/bin/bash

echo "injected endpoint is $ENDPOINT"

RESPONSE=`curl -s $ENDPOINT/mypath`

if [[ "$RESPONSE" != *"Hello from complex docker"* ]]; then
    echo "Unexpected response: $RESPONSE"
    exit 1
fi

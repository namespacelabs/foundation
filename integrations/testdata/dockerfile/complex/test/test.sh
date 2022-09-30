#!/bin/bash

# "-r" removes quotes from the output.
ENDPOINT=`cat /namespace/config/runtime.json | jq -r ".stack_entry[0].service[0].endpoint"`

RESPONSE=`curl -s $ENDPOINT/mypath`

if [[ "$RESPONSE" != *"Hello from complex docker"* ]]; then
    echo "Unexpected response: $RESPONSE"
    exit 1
fi

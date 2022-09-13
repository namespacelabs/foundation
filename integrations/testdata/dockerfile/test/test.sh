#!/bin/bash

# "-r" removes quotes from the output.
ENDPOINT=`cat /namespace/config/runtime.json | jq -r ".stack_entry[0].service[0].endpoint"`

RESPONSE=`curl -s $ENDPOINT`

if [[ "$RESPONSE" != *"Hello,"* ]]; then
    echo "Unexpected response: $RESPONSE"
    exit 1
fi

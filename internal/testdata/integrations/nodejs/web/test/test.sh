#!/bin/bash
set -e

# "-r" removes quotes from the output.
ENDPOINT=`cat /namespace/config/runtime.json | jq -r ".stack_entry[0].service[0].endpoint"`

echo "Extracted endpoint $ENDPOINT"

STATUS=`curl -w '%{http_code}' -o response.txt $ENDPOINT`
if [[ $STATUS -ne 200 ]]; then
    echo "failed to fetch the home page (status $STATUS)"
    cat response.txt
    exit 1
fi

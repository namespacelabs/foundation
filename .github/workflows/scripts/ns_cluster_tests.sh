#!/bin/bash
set -ex

NSC_BIN=$1

ssh-keygen -t rsa -b 4096 -C "gh-action@cluster" -q -N "" -f /tmp/cluster_key
eval `ssh-agent -s`
ssh-add /tmp/cluster_key

# Test ns cluster create
# use old API to pass a custom SSH key.
$NSC_BIN create --output_to /tmp/cluster_id --ssh_key /tmp/cluster_key.pub --compute_api=false
CLUSTER_ID=$(cat /tmp/cluster_id)

# Test ns cluster list
$NSC_BIN list -o json | grep $CLUSTER_ID

# Test ns cluster ssh
tmux new-session -d -s NsSSHSession "$NSC_BIN ssh $CLUSTER_ID"
sleep 5
tmux send-keys -t NsSSHSession "uname -a" Enter
sleep 5
tmux capture-pane -t NsSSHSession -p | grep "Linux $CLUSTER_ID"
tmux send-keys -t NsSSHSession "exit" Enter

# Test ns cluster logs
s=1
for i in $(seq 1 10); do
    LOGS=$($NSC_BIN logs $CLUSTER_ID | wc -l)
    if [[ $(($LOGS)) -gt 0 ]]; then
        echo "Found cluster logs!"
        s=0
        break
    fi
    echo "Still no logs from cluster..."
    sleep 5
done
if [[ $s -gt 0 ]]; then
    exit 1
fi

# Test nsc kubectl get pods -A
s=1
for i in $(seq 1 5); do
    K_OUT=$($NSC_BIN kubectl $CLUSTER_ID get pods -A | wc -l)
    if [[ $(($K_OUT)) -gt 0 ]]; then
        echo "Found K8s pods!"
        s=0
        break
    fi
    echo "Still no pods from cluster..."
    sleep 5
done
if [[ $s -gt 0 ]]; then
    exit 1
fi

# Test ns cluster destroy
$NSC_BIN cluster destroy $CLUSTER_ID --force

# Test ns cluster history
INSTANCE_COUNT=$($NSC_BIN cluster history -o json --since 24h | jq length)
if [[ $((INSTANCE_COUNT)) -lt 1 ]]; then
    echo "no history found!"
    exit 1
fi

ssh-add -d /tmp/cluster_key

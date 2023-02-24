#!/bin/bash
set -ex

NS_BIN=$1

ssh-keygen -t rsa -b 4096 -C "gh-action@cluster" -q -N "" -f /tmp/cluster_key
eval `ssh-agent -s`
ssh-add /tmp/cluster_key

# Test ns cluster create
$NS_BIN cluster create --ephemeral --output_to /tmp/cluster_id --ssh_key /tmp/cluster_key.pub
CLUSTER_ID=$(cat /tmp/cluster_id)

# Test ns cluster list
$NS_BIN cluster list --raw_output | grep $CLUSTER_ID

# Test ns cluster ssh
tmux new-session -d -s NsSSHSession "$NS_BIN cluster ssh $CLUSTER_ID"
sleep 5
tmux send-keys -t NsSSHSession "uname -a" Enter
sleep 5
tmux capture-pane -t NsSSHSession -p | grep "Linux $CLUSTER_ID"
tmux send-keys -t NsSSHSession "exit" Enter

# Test ns cluster logs
s=1
for i in $(seq 1 10); do
    LOGS=$($NS_BIN cluster logs $CLUSTER_ID | wc -l)
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

# Test ns cluster destroy
$NS_BIN cluster destroy $CLUSTER_ID --force

# Test ns cluster history
$NS_BIN cluster history --raw_output | grep $CLUSTER_ID

ssh-add -d /tmp/cluster_key
#!/bin/bash
set -ex

NSC_BIN=$1

ssh-keygen -t ed25519 -C "gh-action@cluster" -q -N "" -f /tmp/cluster_key
eval `ssh-agent -s`
ssh-add /tmp/cluster_key

CI_LABEL="ci-run=${GITHUB_RUN_ID:-unknown}"

cleanup() {
    if [ -f /tmp/cluster_id ]; then
        $NSC_BIN cluster destroy $(cat /tmp/cluster_id) --force || true
    fi
    ssh-add -d /tmp/cluster_key || true
}
trap cleanup EXIT

# Test ns cluster create
# use old API to pass a custom SSH key.
$NSC_BIN create --output_to /tmp/cluster_id --ssh_key /tmp/cluster_key.pub --compute_api=false --label $CI_LABEL
CLUSTER_ID=$(cat /tmp/cluster_id)

# Test ns cluster list
$NSC_BIN list -o json --label $CI_LABEL | tee /tmp/nsc_list.txt
grep $CLUSTER_ID /tmp/nsc_list.txt

# Test ns cluster ssh
tmux new-session -d -s NsSSHSession "$NSC_BIN ssh $CLUSTER_ID"
s=1
for i in $(seq 1 10); do
    if tmux capture-pane -t NsSSHSession -p | grep "$CLUSTER_ID"; then
        s=0
        break
    fi
    sleep 3
done
if [[ $s -gt 0 ]]; then
    echo "FAILED: SSH session did not connect to $CLUSTER_ID."
    tmux capture-pane -t NsSSHSession -p || true
    exit 1
fi
tmux send-keys -t NsSSHSession "uname -a" Enter
sleep 3
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
    echo "FAILED: no logs from $CLUSTER_ID."
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
    echo "FAILED: no Kubernetes pods running on $CLUSTER_ID."
    exit 1
fi

# Test ns cluster destroy
$NSC_BIN cluster destroy $CLUSTER_ID --force

# Test ns cluster history
$NSC_BIN cluster history -o json --since 24h --label $CI_LABEL | tee /tmp/nsc_history.txt
if ! grep $CLUSTER_ID /tmp/nsc_history.txt; then
    echo "FAILED: destroyed instance $CLUSTER_ID not found in history."
    exit 1
fi

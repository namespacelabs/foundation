#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"

bazel_bin=${BAZEL:-bazel}
"$bazel_bin" run //:gazelle -- -r=false internal/testdata/integrations/bazel

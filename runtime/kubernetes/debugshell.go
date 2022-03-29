// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"os/exec"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func (r boundEnv) DebugShell(ctx context.Context, img oci.ImageID, io rtypes.IO) error {
	kubectlBin, err := kubectl.EnsureSDK(ctx)
	if err != nil {
		return err
	}

	done := tasks.EnterInputMode(ctx)
	defer done()

	kubectl := exec.CommandContext(ctx, string(kubectlBin), "--kubeconfig="+r.hostEnv.Kubeconfig, "--context="+r.hostEnv.Context,
		"run", "-n", r.ns(), "-i", "--tty", "--rm", "debug", "--image="+img.ImageRef(), "--restart=Never", "--", "bash")
	kubectl.Stdout = io.Stdout
	kubectl.Stderr = io.Stderr
	kubectl.Stdin = io.Stdin

	return kubectl.Run()
}
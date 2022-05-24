// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"os/exec"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/runtime/rtypes"
)

func (r boundEnv) Kubectl(ctx context.Context, io rtypes.IO, args ...string) error {
	kubectlBin, err := kubectl.EnsureSDK(ctx)
	if err != nil {
		return err
	}

	done := console.EnterInputMode(ctx)
	defer done()

	kubectl := exec.CommandContext(ctx, string(kubectlBin),
		append([]string{"--kubeconfig=" + r.hostEnv.Kubeconfig, "--context=" + r.hostEnv.Context, "-n", r.moduleNamespace}, args...)...)
	kubectl.Stdout = io.Stdout
	kubectl.Stderr = io.Stderr
	kubectl.Stdin = io.Stdin
	return kubectl.Run()
}

type KubeConfig struct {
	Config, Context, Namespace string
}

func (r boundEnv) KubeConfig() KubeConfig {
	return KubeConfig{
		Config:    r.hostEnv.Kubeconfig,
		Context:   r.hostEnv.Context,
		Namespace: r.moduleNamespace,
	}
}

func (r boundEnv) DebugShell(ctx context.Context, img oci.ImageID, io rtypes.IO) error {
	return r.Kubectl(ctx, io, "run", "-i", "--tty", "--rm", "debug", "--image="+img.ImageRef(), "--restart=Never", "--", "bash")
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

func (r k8sRuntime) RunController(ctx context.Context, runOpts runtime.ServerRunOpts) error {
	name := fmt.Sprintf("controller-%v", labelName(runOpts.Command))

	container := applycorev1.Container().
		WithName(name).
		WithImage(runOpts.Image.RepoAndDigest()).
		WithArgs(runOpts.Args...).
		WithCommand(runOpts.Command...).
		WithSecurityContext(
			applycorev1.SecurityContext().
				WithReadOnlyRootFilesystem(runOpts.ReadOnlyFilesystem))

	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return err
	}

	pod := applycorev1.Pod(name, r.ns()).
		WithSpec(applycorev1.PodSpec().WithContainers(container).
			WithRestartPolicy(corev1.RestartPolicyOnFailure))

	if _, err := cli.CoreV1().Pods(r.ns()).Apply(ctx, pod, kubedef.Ego()); err != nil {
		return err
	}

	// Shall we block on the controller becoming healthy?

	return nil
}

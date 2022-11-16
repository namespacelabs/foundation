// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/std/cfg"
)

func newExistingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "existing",
		Short: "Prepares a Namespace enviroment with an existing Kubernetes cluster.",
	}

	contextName := cmd.Flags().String("context", "", "The name of the kubernetes context to use.")
	registryAddr := cmd.Flags().String("registry", "", "Which registry to use for deployed images.")
	kubeConfig := cmd.Flags().String("kube_config", "~/.kube/config", "Which kubernetes configuration to use.")
	insecure := cmd.Flags().Bool("insecure", false, "Set to true if the image registry is not accessible via TLS.")

	return fncobra.With(cmd, func(ctx context.Context) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		env, err := cfg.LoadContext(root, envRef)
		if err != nil {
			return err
		}

		if *contextName == "" {
			return fnerrors.New("--context is required; it's the name of an existing kubernetes context")
		}

		if *registryAddr == "" {
			return fnerrors.New("--registry is required; it's the url of an existing image registry")
		}

		insecureLabel := ""
		if *insecure {
			insecureLabel = fmt.Sprintf(" insecure=%v", *insecure)
		}

		fmt.Fprintf(console.Stdout(ctx), "Setting up existing cluster configured at context %q (registry %q%s)...\n", *contextName, *registryAddr, insecureLabel)

		k8sconfig := prepare.PrepareExistingK8s(env, *kubeConfig, *contextName, &registry.Registry{
			Url:      *registryAddr,
			Insecure: *insecure,
		})

		return collectPreparesAndUpdateDevhost(ctx, root, envRef, prepare.PrepareCluster(env, k8sconfig))
	})
}

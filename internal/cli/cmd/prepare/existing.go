// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
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
	useDockerCredentials := cmd.Flags().Bool("use_docker_creds", false, "If set to true, uses Docker's credentials when accessing the registry.")
	singleRepository := cmd.Flags().Bool("use_single_repository", false, "If set to true, collapse all images onto a single repository under the configured registry (rather than a repository per image).")

	cmd.RunE = runPrepare(func(ctx context.Context, env cfg.Context) (compute.Computable[*schema.DevHost_ConfigureEnvironment], error) {
		if *contextName == "" {
			return nil, fnerrors.New("--context is required; it's the name of an existing kubernetes context")
		}

		if *registryAddr == "" {
			return nil, fnerrors.New("--registry is required; it's the url of an existing image registry")
		}

		repo, err := name.NewRepository(*registryAddr)
		if err != nil {
			return nil, fnerrors.New("invalid registry definition: %w", err)
		}

		// Docker Hub validation.
		if repo.Registry.Name() == name.DefaultRegistry {
			warn := console.Warnings(ctx)

			fmt.Fprintf(warn, `
  Docker Hub detected as the target registry.

  When using Docker Hub, we collapse all image pushes to a single repository,
  as Docker Hub doesn't support multi segment repositories.

  Also, the configured repository must be public, as we don't forward
  your docker credentials to the cluster.

`)

			if !*singleRepository {
				return nil, fnerrors.New("--use_single_repository is required when the target registry is Docker Hub")
			}

			if !*useDockerCredentials {
				return nil, fnerrors.New("--use_docker_creds is required when the target registry is Docker Hub")
			}

			parts := strings.Split(repo.RepositoryStr(), "/")
			if len(parts) != 2 || parts[0] == "library" {
				return nil, fnerrors.New("when using Docker Hub, you must specify the target registry explicitly. E.g. --registry docker.io/username/namespace-images")
			}
		}

		cfg, err := client.LoadExistingConfiguration(*kubeConfig, *contextName)
		if err != nil {
			return nil, fnerrors.New("failed to load existing configuration: %w", err)
		}

		if _, err := cfg.ClientConfig(); err != nil {
			return nil, fnerrors.New("failed to load existing configuration from %q: %w (you can change which kubeconfig is used via --kube_config)", *kubeConfig, err)
		}

		insecureLabel := ""
		if *insecure {
			insecureLabel = fmt.Sprintf(" insecure=%v", *insecure)
		}
		fmt.Fprintf(console.Stdout(ctx), "Setting up existing cluster configured at context %q (registry %q%s)...\n", *contextName, *registryAddr, insecureLabel)

		k8sconfig := prepare.PrepareExistingK8s(env, *kubeConfig, *contextName, &registry.Registry{
			Url:              *registryAddr,
			Insecure:         *insecure,
			UseDockerAuth:    *useDockerCredentials,
			SingleRepository: *singleRepository,
		})

		return prepare.PrepareCluster(env, k8sconfig), nil
	})

	return cmd
}

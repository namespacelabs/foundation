// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
)

func NewCreateCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "create",
		Short:  "Creates a new cluster.",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type.")
	ephemeral := cmd.Flags().Bool("ephemeral", true, "Create an ephemeral cluster.")
	features := cmd.Flags().StringSlice("features", nil, "A set of features to attach to the cluster.")
	waitKubeSystem := cmd.Flags().Bool("wait_kube_system", false, "If true, wait until kube-system resources (e.g. coredns and local-path-provisioner) are ready.")

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the cluster id to this path.")
	outputJsonPath := cmd.Flags().String("output_json_to", "", "If specified, write cluster metadata as JSON to this path.")
	outputRegistryPath := cmd.Flags().String("output_registry_to", "", "If specified, write the registry address to this path.")

	userSshey := cmd.Flags().String("ssh_key", "", "Injects the specified ssh public key in the created cluster.")

	internalExtra := cmd.Flags().String("internal_extra", "", "Internal creation details.")
	cmd.Flags().MarkHidden("internal_extra")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		opts := api.CreateClusterOpts{
			MachineType:   *machineType,
			Ephemeral:     *ephemeral,
			KeepAtExit:    true,
			Purpose:       "Manually created from CLI",
			Features:      *features,
			InternalExtra: *internalExtra,
		}

		if *userSshey != "" {
			keyData, err := os.ReadFile(*userSshey)
			if err != nil {
				return fnerrors.New("failed to load key: %w", err)
			}

			opts.AuthorizedSshKeys = append(opts.AuthorizedSshKeys, string(keyData))
		}

		opts.WaitKind = "kubernetes"

		cluster, err := api.CreateAndWaitCluster(ctx, api.Endpoint, opts)
		if err != nil {
			return err
		}

		if *waitKubeSystem {
			if err := ctl.WaitKubeSystem(ctx, cluster.Cluster); err != nil {
				return err
			}
		}

		if *outputPath != "" {
			if err := os.WriteFile(*outputPath, []byte(cluster.ClusterId), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputPath, err)
			}
		}

		if *outputRegistryPath != "" {
			if err := os.WriteFile(*outputRegistryPath, []byte(cluster.Registry.EndpointAddress), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputRegistryPath, err)
			}
		}

		if *outputJsonPath != "" {
			// Clear out secrets from output.
			copy := *cluster.Cluster
			copy.SshPrivateKey = nil
			copy.CertificateAuthorityData = nil
			copy.ClientCertificateData = nil
			copy.ClientKeyData = nil

			serialized, err := json.MarshalIndent(copy, "", "  ")
			if err != nil {
				return fnerrors.New("failed to serialize: %v", err)
			}

			if err := os.WriteFile(*outputJsonPath, serialized, 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputJsonPath, err)
			}
		}

		printNewEnv(ctx, cluster.ClusterId, cluster.Cluster.AppURL)

		if api.ClusterService(cluster.Cluster, "kubernetes") != nil {
			stdout := console.Stdout(ctx)
			style := colors.Ctx(ctx)
			fmt.Fprintln(stdout)
			fmt.Fprintf(stdout, "  As a next step, try one of:\n\n")
			fmt.Fprintf(stdout, "    $ nsc kubectl %s get pod -A\n\n", cluster.ClusterId)
			fmt.Fprintf(stdout, "    $ nsc kubeconfig write %s\n", cluster.ClusterId)
			fmt.Fprintf(stdout, "      %s\n", style.Comment.Apply("<follow instructions>"))
			fmt.Fprintf(stdout, "    $ kubectl get pod -A\n\n")
			fmt.Fprintf(stdout, "  You can also connect to a shell in the new environment:\n\n")
			fmt.Fprintf(stdout, "    $ nsc ssh %s\n\n", cluster.ClusterId)
		}

		return nil
	})

	return cmd
}

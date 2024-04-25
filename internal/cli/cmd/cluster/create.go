// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
)

func NewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new instance.",
		Args:  cobra.NoArgs,
	}

	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type.")
	unusedEphemeral := cmd.Flags().Bool("ephemeral", false, "Create an ephemeral instance.")
	features := cmd.Flags().StringSlice("features", nil, "A set of features to attach to the instance.")
	waitKubeSystem := cmd.Flags().Bool("wait_kube_system", false, "If true, wait until kube-system resources (e.g. coredns and local-path-provisioner) are ready.")
	bare := cmd.Flags().Bool("bare", false, "If set to true, creates an environment with the minimal set of services (e.g. no Kubernetes).")
	tag := cmd.Flags().String("unique_tag", "", "If specified, creates a instance with the specified unique tag.")
	labels := cmd.Flags().StringToString("label", nil, "Key-values to attach to the new instance. Multiple key-value pairs may be specified.")

	legacyOutputPath := cmd.Flags().String("output_to", "", "If specified, write the instance id to this path.")
	cmd.Flags().MarkDeprecated("output_to", "use cidfile instead")
	cidfile := cmd.Flags().String("cidfile", "", "If specified, write the instance id to this path.")
	outputJsonPath := cmd.Flags().String("output_json_to", "", "If specified, write instance metadata as JSON to this path.")
	outputRegistryPath := cmd.Flags().String("output_registry_to", "", "If specified, write the registry address to this path.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	userSshey := cmd.Flags().String("ssh_key", "", "Injects the specified ssh public key in the created instance.")
	cmd.Flags().MarkHidden("ssh_key")
	experimental := cmd.Flags().String("experimental", "", "JSON definition of experimental features.")
	experimentalFrom := cmd.Flags().String("experimental_from", "", "Load experimental definitions from the specified file.")

	duration := cmd.Flags().Duration("duration", 0, "For how long to run the ephemeral environment.")

	availableSecrets := cmd.Flags().StringSlice("available_secrets", nil, "Attaches the specified secrets to this instance.")
	cmd.Flags().MarkHidden("available_secrets")

	internalExtra := cmd.Flags().String("internal_extra", "", "Internal creation details.")
	cmd.Flags().MarkHidden("internal_extra")

	volumes := cmd.Flags().StringSlice("volume", nil, "Attach a volume to the instance, {cache|persistent}:{tag}:{mountpoint}:{size}")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *unusedEphemeral {
			fmt.Fprintf(console.Warnings(ctx), "--ephemeral has been removed and does impact the creation request (try --machine_type instead)")
		}

		opts := api.CreateClusterOpts{
			MachineType:   *machineType,
			KeepAtExit:    true,
			Purpose:       "Manually created from CLI",
			Features:      *features,
			InternalExtra: *internalExtra,
			Labels:        *labels,
			UniqueTag:     *tag,
			SecretIDs:     *availableSecrets,
		}

		if len(opts.Labels) == 0 {
			opts.Labels = map[string]string{
				"nsc.source": "nsc",
			}
		}

		if *duration > 0 {
			opts.Deadline = timestamppb.New(time.Now().Add(*duration))
		}

		if *userSshey != "" {
			keyData, err := os.ReadFile(*userSshey)
			if err != nil {
				return fnerrors.New("failed to load key: %w", err)
			}

			for _, line := range strings.Split(string(keyData), "\n") {
				if clean := strings.TrimSpace(line); clean != "" {
					opts.AuthorizedSshKeys = append(opts.AuthorizedSshKeys, clean)
				}
			}
		}

		if *experimental != "" && *experimentalFrom != "" {
			return fnerrors.New("must only set one of --experimental or --experimental_from")
		}

		if *experimental != "" {
			var exp any
			if err := json.Unmarshal([]byte(*experimental), &exp); err != nil {
				return err
			}
			opts.Experimental = exp
		}

		if *experimentalFrom != "" {
			var exp any
			if err := files.ReadJson(*experimentalFrom, &exp); err != nil {
				return err
			}

			opts.Experimental = exp
		}

		for _, def := range *volumes {
			parts := strings.Split(def, ":")
			if len(parts) != 3 && len(parts) != 4 {
				return fnerrors.New("failed to parse volume definition: ")
			}

			kind := parts[0]
			tag := parts[1]
			mountPoint := parts[2]

			var sizeMb int64
			if len(parts) == 4 {
				sz, err := units.RAMInBytes(parts[3])
				if err != nil {
					return fnerrors.New("failed to parse size: %w", err)
				}

				sizeMb = sz / (1024 * 1024)
			}

			for _, t := range []struct {
				key, val string
			}{
				{"tag", tag},
				{"mount_point", mountPoint},
				{"kind", kind},
			} {
				if t.val == "" {
					return fnerrors.New("a volume %q is required", t.key)
				}
			}

			spec := api.VolumeSpec{
				Tag:        tag,
				SizeMb:     sizeMb,
				MountPoint: mountPoint,
			}

			switch strings.ToLower(kind) {
			case "cache":
				spec.PersistencyKind = api.VolumeSpec_CACHE
			case "persistent":
				spec.PersistencyKind = api.VolumeSpec_PERSISTENT
			default:
				return fnerrors.New("a volume %q of %q or %q is required", "kind", "cache", "persistent")
			}

			opts.Volumes = append(opts.Volumes, spec)
		}

		if *bare {
			opts.Features = append(opts.Features, "EXP_DISABLE_KUBERNETES")
		}

		opts.WaitKind = "kubernetes"

		cluster, err := api.CreateAndWaitCluster(ctx, api.Methods, opts)
		if err != nil {
			return err
		}

		if *waitKubeSystem {
			if err := ctl.WaitKubeSystem(ctx, cluster.Cluster); err != nil {
				return err
			}
		}

		if *legacyOutputPath != "" {
			if err := os.WriteFile(*legacyOutputPath, []byte(cluster.ClusterId), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *legacyOutputPath, err)
			}
		}

		if *cidfile != "" {
			if err := os.WriteFile(*cidfile, []byte(cluster.ClusterId), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *cidfile, err)
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

		switch *output {
		case "json":
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")

			if err := enc.Encode(createOutput{
				ClusterId:     cluster.ClusterId,
				ClusterUrl:    cluster.Cluster.AppURL,
				IngressDomain: cluster.Cluster.IngressDomain,
			}); err != nil {
				return fnerrors.InternalError("failed to encode instance as JSON output: %w", err)
			}

		default:
			if *output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "defaulting output to plain\n")
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

			if len(cluster.Cluster.TlsBackedPort) > 0 {
				stdout := console.Stdout(ctx)
				fmt.Fprintln(stdout)
				fmt.Fprintf(stdout, "  (Experimental) TLS backend ports:\n\n")
				for _, port := range cluster.Cluster.TlsBackedPort {
					fmt.Fprintf(stdout, "    %s (%s/%d)\n", port.ServerName, port.Name, port.Port)
				}
				fmt.Fprintln(stdout)
			}
		}

		return nil
	})

	return cmd
}

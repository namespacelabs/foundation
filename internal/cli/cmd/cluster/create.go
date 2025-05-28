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
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/private"
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
	bare := cmd.Flags().Bool("bare", false, "If set to true, creates an environment with the minimal set of services (e.g. no Kubernetes).")
	tag := cmd.Flags().String("unique_tag", "", "If specified, creates a instance with the specified unique tag.")
	labels := cmd.Flags().StringToString("label", nil, "Key-values to attach to the new instance. Multiple key-value pairs may be specified.")
	purpose := cmd.Flags().String("purpose", "Manually created from CLI", "What documented purpose to attach to the created instance.")

	ingress := cmd.Flags().String("ingress", "", "If set, configures the ingress of this instance. Valid options: wildcard.")

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

	waitKubeSystem := cmd.Flags().Bool("wait_kube_system", false, "If true, wait until kube-system resources (e.g. coredns and local-path-provisioner) are ready.")
	waitTimeout := cmd.Flags().Duration("wait_timeout", time.Minute, "For how long to wait until the instance becomes ready.")

	availableSecrets := cmd.Flags().StringSlice("available_secrets", nil, "Attaches the specified secrets to this instance.")
	cmd.Flags().MarkHidden("available_secrets")

	internalExtra := cmd.Flags().String("internal_extra", "", "Internal creation details.")
	cmd.Flags().MarkHidden("internal_extra")

	volumes := cmd.Flags().StringSlice("volume", nil, "Attach a volume to the instance, {cache|persistent}:{tag}:{mountpoint}:{size}")

	// Leave it on purpose so it's backwards compatible.
	_ = cmd.Flags().Bool("compute_api", true, "Whether to use the Compute API.")
	cmd.Flags().MarkHidden("compute_api")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *unusedEphemeral {
			fmt.Fprintf(console.Warnings(ctx), "--ephemeral has been removed and does impact the creation request (try --machine_type instead)")
		}

		opts := api.CreateClusterOpts{
			MachineType: *machineType,
			KeepAtExit:  true,
			Purpose:     *purpose,
			Features:    *features,
			Labels:      *labels,
			UniqueTag:   *tag,
			SecretIDs:   *availableSecrets,
			Duration:    *duration,
		}

		if len(opts.Labels) == 0 {
			opts.Labels = map[string]string{
				"nsc.source": "nsc",
			}
		}

		if *experimental != "" && *experimentalFrom != "" {
			return fnerrors.Newf("must only set one of --experimental or --experimental_from")
		}

		if *experimental != "" {
			var exp map[string]any
			if err := json.Unmarshal([]byte(*experimental), &exp); err != nil {
				return err
			}

			opts.Experimental = exp
		}

		if *experimentalFrom != "" {
			var exp map[string]any
			if err := files.ReadJson(*experimentalFrom, &exp); err != nil {
				return err
			}

			opts.Experimental = exp
		}

		if *internalExtra != "" {
			if opts.Experimental == nil {
				opts.Experimental = map[string]any{}
			}

			opts.Experimental["internal_extra"] = *internalExtra
		}

		if keys, err := parseAuthorizedKeys(*userSshey); err != nil {
			return err
		} else {
			if opts.Experimental == nil {
				opts.Experimental = map[string]any{}
			}

			opts.Experimental["authorized_ssh_keys"] = keys
		}

		for _, def := range *volumes {
			parts := strings.Split(def, ":")
			if len(parts) != 3 && len(parts) != 4 {
				return fnerrors.Newf("failed to parse volume definition")
			}

			kind := parts[0]
			tag := parts[1]
			mountPoint := parts[2]

			var sizeMb int64
			if len(parts) == 4 {
				sz, err := units.RAMInBytes(parts[3])
				if err != nil {
					return fnerrors.Newf("failed to parse size: %w", err)
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
					return fnerrors.Newf("a volume %q is required", t.key)
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
				return fnerrors.Newf("a volume %q of %q or %q is required", "kind", "cache", "persistent")
			}

			opts.Volumes = append(opts.Volumes, spec)
		}

		if *bare {
			opts.Features = append(opts.Features, "EXP_DISABLE_KUBERNETES")
		}

		// XXX hacky for backwards compatibility.
		if opts.Experimental == nil && !*bare && !strings.HasPrefix(*machineType, "mac") {
			opts.Experimental = map[string]any{
				"k3s": private.K3sCfg,
			}
		}

		switch *ingress {
		case "":
			// nothing to do

		case "wildcard":
			opts.Features = append(opts.Features, "EXP_REGISTER_INSTANCE_WILDCARD_CERT")

		default:
			return fnerrors.Newf("unknown ingress option %q", *ingress)
		}

		opts.WaitKind = "kubernetes"

		cluster, err := api.CreateAndWaitCluster(ctx, api.Methods, *waitTimeout, opts)
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
				return fnerrors.Newf("failed to write %q: %w", *legacyOutputPath, err)
			}
		}

		if *cidfile != "" {
			if err := os.WriteFile(*cidfile, []byte(cluster.ClusterId), 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", *cidfile, err)
			}
		}

		if *outputRegistryPath != "" {
			reg, err := api.GetImageRegistry(ctx, api.Methods)
			if err != nil {
				return fnerrors.Newf("failed to fetch registry: %w", err)
			}

			if err := os.WriteFile(*outputRegistryPath, []byte(reg.NSCR.EndpointAddress), 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", *outputRegistryPath, err)
			}
		}

		if *outputJsonPath != "" {
			if cluster.Cluster == nil {
				return fnerrors.New("no instance information available with no wait timeout")
			}

			// Clear out secrets from output.
			copy := *cluster.Cluster
			copy.SshPrivateKey = nil
			copy.CertificateAuthorityData = nil
			copy.ClientCertificateData = nil
			copy.ClientKeyData = nil

			serialized, err := json.MarshalIndent(copy, "", "  ")
			if err != nil {
				return fnerrors.Newf("failed to serialize: %v", err)
			}

			if err := os.WriteFile(*outputJsonPath, serialized, 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", *outputJsonPath, err)
			}
		}

		switch *output {
		case "json":
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")

			out := createOutput{
				ClusterId: cluster.ClusterId,
			}

			if cluster.Cluster != nil {
				out.ClusterUrl = cluster.Cluster.AppURL
				out.IngressDomain = cluster.Cluster.IngressDomain
				out.ApiEndpoint = cluster.Cluster.ApiEndpoint
			}

			if err := enc.Encode(out); err != nil {
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

			if cluster.Cluster != nil && len(cluster.Cluster.TlsBackedPort) > 0 {
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

func parseAuthorizedKeys(file string) ([]string, error) {
	if file == "" {
		return nil, nil
	}

	keyData, err := os.ReadFile(file)
	if err != nil {
		return nil, fnerrors.Newf("failed to load keys: %w", err)
	}

	var keys []string
	for _, line := range strings.Split(string(keyData), "\n") {
		if clean := strings.TrimSpace(line); clean != "" {
			keys = append(keys, clean)
		}
	}

	return keys, nil
}

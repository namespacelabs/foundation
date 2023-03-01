package cluster

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new cluster.",
		Args:  cobra.NoArgs,
	}

	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type.")
	ephemeral := cmd.Flags().Bool("ephemeral", false, "Create an ephemeral cluster.")
	features := cmd.Flags().StringSlice("features", nil, "A set of features to attach to the cluster.")
	waitKubeSystem := cmd.Flags().Bool("wait_kube_system", false, "If true, wait until kube-system resources (e.g. coredns and local-path-provisioner) are ready.")

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the cluster id to this path.")
	outputRegistryPath := cmd.Flags().String("output_registry_to", "", "If specified, write the registry address to this path.")

	userSshey := cmd.Flags().String("ssh_key", "", "Injects the specified ssh public key in the created cluster.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		opts := api.CreateClusterOpts{
			MachineType: *machineType,
			Ephemeral:   *ephemeral,
			KeepAlive:   true,
			Purpose:     "Manually created from CLI",
			Features:    *features,
		}

		if *userSshey != "" {
			keyData, err := os.ReadFile(*userSshey)
			if err != nil {
				return fnerrors.New("failed to load key: %w", err)
			}

			opts.AuthorizedSshKeys = append(opts.AuthorizedSshKeys, string(keyData))
		}

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

		stdout := console.Stdout(ctx)
		fmt.Fprintf(stdout, "Created cluster %q\n", cluster.ClusterId)
		if cluster.Deadline != nil {
			fmt.Fprintf(stdout, " deadline: %s\n", cluster.Deadline.Format(time.RFC3339))
		} else {
			fmt.Fprintf(stdout, " no deadline\n")
		}
		return nil
	})

	return cmd
}

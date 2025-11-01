// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package scratch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [command]",
		Short: "Create a temporary scratch instance and SSH to it. The instance is destroyed on exit.",
		Args:  cobra.ArbitraryArgs,
	}

	sshAgent := cmd.Flags().BoolP("ssh_agent", "A", false, "If specified, forwards the local SSH agent.")
	forcePty := cmd.Flags().BoolP("force-pty", "t", false, "Force pseudo-terminal allocation.")
	disablePty := cmd.Flags().BoolP("disable-pty", "T", false, "Disable pseudo-terminal allocation.")
	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type.")
	waitTimeout := cmd.Flags().Duration("wait_timeout", 2*time.Minute, "For how long to wait until the instance becomes ready.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *forcePty && *disablePty {
			return errors.New("Can not use -t and -T")
		}

		sshOpts := cluster.InlineSshOpts{
			ForwardSshAgent: *sshAgent,
			ForcePty:        *forcePty,
			DisablePty:      *disablePty,
		}

		fmt.Fprintln(console.Stdout(ctx), "Creating temporary scratch instance...")

		// Create a temporary instance for SSH
		opts := api.CreateClusterOpts{
			MachineType: *machineType,
			KeepAtExit:  false, // Destroy on exit
			Purpose:     "Temporary scratch instance for SSH",
			WaitClusterOpts: api.WaitClusterOpts{
				WaitForService: "ssh",
				WaitKind:       "kubernetes",
			},
			Duration: 5 * time.Minute, // Initial duration, will be kept alive during SSH session
		}

		clusterResp, err := api.CreateAndWaitCluster(ctx, api.Methods, *waitTimeout, opts)
		if err != nil {
			return err
		}

		if clusterResp.Cluster == nil {
			return fnerrors.InternalError("cluster response is missing cluster information")
		}

		fmt.Fprintf(console.Stdout(ctx), "Connected to instance %s\n", clusterResp.Cluster.ClusterId)
		fmt.Fprintln(console.Stdout(ctx), "Instance will be destroyed when you exit the SSH session.")

		// SSH to the instance. The InlineSsh function handles keeping the instance alive
		// via api.StartRefreshing during the SSH session.
		err = cluster.InlineSsh(ctx, clusterResp.Cluster, sshOpts, args)

		// Instance will be automatically destroyed since KeepAtExit is false
		fmt.Fprintln(console.Stdout(ctx), "\nDestroying scratch instance...")

		return err
	})

	return cmd
}

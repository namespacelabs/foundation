// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gorilla/websocket"
	"github.com/jpillora/chisel/share/cnet"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/providers/nscloud"
)

func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "cluster",
		Short:  "Cluster-related activities (internal only).",
		Hidden: true,
	}

	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newSshCmd())

	return cmd
}

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a new cluster.",
		Args:  cobra.NoArgs,
	}

	machineType := cmd.Flags().String("machine_type", "", "Specify the machine type.")
	ephemeral := cmd.Flags().Bool("ephemeral", false, "Create an ephemeral cluster.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, err := nscloud.CreateAndWaitCluster(ctx, *machineType, *ephemeral, "manually created")
		if err != nil {
			return err
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

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all of your clusters.",
		Args:  cobra.NoArgs,
	}

	rawOutput := cmd.Flags().Bool("raw_output", false, "Dump the resulting server response, without formatting.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusters, err := nscloud.ListClusters(ctx)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		if *rawOutput {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(clusters)
		} else {
			for _, cluster := range clusters.Clusters {
				fmt.Fprintf(stdout, "%s %s\n", cluster.ClusterId, formatDescription(cluster))
			}
		}

		return nil
	})

	return cmd
}

func newSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh {cluster-id}",
		Short: "Start an SSH session.",
		Args:  cobra.MaximumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if len(args) > 0 {
			cluster, err := nscloud.GetCluster(ctx, args[0])
			if err != nil {
				return err
			}

			return ssh(ctx, *cluster, args[1:])
		}

		clusters, err := nscloud.ListClusters(ctx)
		if err != nil {
			return err
		}

		var cls []cluster
		for _, cl := range clusters.Clusters {
			cls = append(cls, cluster(cl))
		}

		cl, err := tui.Select(ctx, "Which cluster would you like to connect to?", cls)
		if err != nil {
			return err
		}

		if cl == nil {
			return nil
		}

		cluster := cl.(cluster)

		return ssh(ctx, cluster.Cluster(), nil)
	})

	return cmd
}

type cluster nscloud.KubernetesCluster

func (d cluster) Cluster() nscloud.KubernetesCluster { return nscloud.KubernetesCluster(d) }
func (d cluster) Title() string                      { return d.ClusterId }
func (d cluster) Description() string                { return formatDescription(nscloud.KubernetesCluster(d)) }
func (d cluster) FilterValue() string                { return d.ClusterId }

func formatDescription(cluster nscloud.KubernetesCluster) string {
	cpu := "<unknown>"
	ram := "<unknown>"

	if shape := cluster.Shape; shape != nil {
		cpu = fmt.Sprintf("%d", shape.VirtualCpu)
		ram = humanize.IBytes(uint64(shape.MemoryMegabytes) * humanize.MiByte)
	}

	created, _ := time.Parse(time.RFC3339, cluster.Created)
	deadline, _ := time.Parse(time.RFC3339, cluster.Deadline)

	return fmt.Sprintf("[cpu: %s ram: %s] (created %v, for %v, dist: %s): %s",
		cpu, ram, created.Local(), time.Until(deadline),
		cluster.KubernetesDistribution, cluster.DocumentedPurpose)
}

func ssh(ctx context.Context, cluster nscloud.KubernetesCluster, args []string) error {
	lst, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := lst.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				d := websocket.Dialer{
					HandshakeTimeout: 15 * time.Second,
				}

				wsConn, _, err := d.DialContext(ctx, cluster.SSHProxyEndpoint, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
					return
				}

				proxyConn := cnet.NewWebSocketConn(wsConn)

				go func() {
					_, _ = io.Copy(conn, proxyConn)
				}()

				_, _ = io.Copy(proxyConn, conn)
			}()
		}
	}()

	localPort := lst.Addr().(*net.TCPAddr).Port

	args = append(args,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "UpdateHostKeys no",
		"-p", fmt.Sprintf("%d", localPort), "janitor@127.0.0.1")

	cmd := exec.CommandContext(ctx, "ssh", args...)
	return localexec.RunInteractive(ctx, cmd)
}

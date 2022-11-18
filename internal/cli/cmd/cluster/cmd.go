// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnnet"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
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
	cmd.AddCommand(newPortForwardCmd())

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
		cluster, err := selectCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		return ssh(ctx, cluster, nil)
	})

	return cmd
}

func newPortForwardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward {cluster-id}",
		Short: "Opens a local port which connects to the cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	port := cmd.Flags().Int("target_port", 0, "Which port to forward to.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *port == 0 {
			return fnerrors.New("--target_port is required")
		}

		cluster, err := selectCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		return portForward(ctx, cluster, *port)
	})

	return cmd
}

func selectCluster(ctx context.Context, args []string) (*nscloud.KubernetesCluster, error) {
	if len(args) > 0 {
		response, err := nscloud.GetCluster(ctx, args[0])
		if err != nil {
			return nil, err
		}
		return response.Cluster, nil
	}

	clusters, err := nscloud.ListClusters(ctx)
	if err != nil {
		return nil, err
	}

	var cls []cluster
	for _, cl := range clusters.Clusters {
		cls = append(cls, cluster(cl))
	}

	cl, err := tui.Select(ctx, "Which cluster would you like to connect to?", cls)
	if err != nil {
		return nil, err
	}

	if cl == nil {
		return nil, nil
	}

	d := cl.(cluster).Cluster()
	return &d, nil
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

func ssh(ctx context.Context, cluster *nscloud.KubernetesCluster, args []string) error {
	lst, err := fnnet.ListenPort(ctx, "127.0.0.1", 0, 0)
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

func portForward(ctx context.Context, cluster *nscloud.KubernetesCluster, targetPort int) error {
	lst, err := fnnet.ListenPort(ctx, "127.0.0.1", 0, targetPort)
	if err != nil {
		return err
	}

	localPort := lst.Addr().(*net.TCPAddr).Port
	fmt.Fprintf(console.Stdout(ctx), "Listening on 127.0.0.1:%d\n", localPort)

	for {
		conn, err := lst.Accept()
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "New connection from %v\n", conn.RemoteAddr())

		go func() {
			defer conn.Close()

			proxyConn, err := nscloud.DialPort(ctx, cluster, targetPort)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
				return
			}

			go func() {
				_, _ = io.Copy(conn, proxyConn)
			}()

			_, _ = io.Copy(proxyConn, conn)
		}()
	}
}

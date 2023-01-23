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
	"namespacelabs.dev/foundation/internal/cli/cmd/tools"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnnet"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
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
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(newKubectlCmd())
	cmd.AddCommand(newKubeconfigCmd())

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
	features := cmd.Flags().StringSlice("features", nil, "A set of features to attach to the cluster.")

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the cluster id to this path.")
	outputRegistryPath := cmd.Flags().String("output_registry_to", "", "If specified, write the registry address to this path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, err := api.CreateAndWaitCluster(ctx, api.Endpoint, api.CreateClusterOpts{
			MachineType: *machineType,
			Ephemeral:   *ephemeral,
			KeepAlive:   true,
			Purpose:     "manually created",
			Features:    *features,
		})
		if err != nil {
			return err
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

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all of your clusters.",
		Args:  cobra.NoArgs,
	}

	rawOutput := cmd.Flags().Bool("raw_output", false, "Dump the resulting server response, without formatting.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusters, err := api.ListClusters(ctx, api.Endpoint)
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
		cluster, _, err := selectCluster(ctx, args)
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

		cluster, _, err := selectCluster(ctx, args)
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

func newDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy {cluster-id}",
		Short: "Destroys an existing cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	force := cmd.Flags().Bool("force", false, "Skip the confirmation step.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := selectCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		if !*force {
			result, err := tui.Ask(ctx, "Do you want to remove this cluster?",
				fmt.Sprintf(`This is a destructive action.

Type %q for it to be removed.`, cluster.ClusterId), "")
			if err != nil {
				return err
			}

			if result != cluster.ClusterId {
				return context.Canceled
			}
		}

		return api.DestroyCluster(ctx, api.Endpoint, cluster.ClusterId)
	})

	return cmd
}

func newKubectlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl -- ...",
		Short: "Run kubectl on the target cluster.",
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, args, err := selectCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		cfg, err := tools.WriteRawKubeconfig(ctx, nscloud.MakeConfig(cluster), false)
		if err != nil {
			return err
		}

		defer func() { _ = cfg.Cleanup() }()

		cmdLine := append(cfg.BaseArgs(), args...)

		kubectlBin, err := kubectl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download Kubernetes SDK: %w", err)
		}

		kubectl := exec.CommandContext(ctx, string(kubectlBin), cmdLine...)
		return localexec.RunInteractive(ctx, kubectl)
	})

	return cmd
}

func newKubeconfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Write Kubeconfig for the target cluster.",
	}

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the path of the Kubeconfig to this path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := selectCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		cfg, err := tools.WriteRawKubeconfig(ctx, nscloud.MakeConfig(cluster), false)
		if err != nil {
			return err
		}

		if *outputPath != "" {
			if err := os.WriteFile(*outputPath, []byte(cfg.Kubeconfig), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputPath, err)
			}
		}

		fmt.Fprintf(console.Stdout(ctx), "Wrote Kubeconfig for cluster %s to %s.\n", cluster.ClusterId, cfg.Kubeconfig)
		return nil
	})

	return cmd
}

func selectCluster(ctx context.Context, args []string) (*api.KubernetesCluster, []string, error) {
	if len(args) > 0 {
		response, err := api.GetCluster(ctx, api.Endpoint, args[0])
		if err != nil {
			return nil, nil, err
		}
		return response.Cluster, args[1:], nil
	}

	clusters, err := api.ListClusters(ctx, api.Endpoint)
	if err != nil {
		return nil, nil, err
	}

	var cls []cluster
	for _, cl := range clusters.Clusters {
		cls = append(cls, cluster(cl))
	}

	cl, err := tui.Select(ctx, "Which cluster would you like to connect to?", cls)
	if err != nil {
		return nil, nil, err
	}

	if cl == nil {
		return nil, nil, nil
	}

	d := cl.(cluster).Cluster()
	return &d, nil, nil
}

type cluster api.KubernetesCluster

func (d cluster) Cluster() api.KubernetesCluster { return api.KubernetesCluster(d) }
func (d cluster) Title() string                  { return d.ClusterId }
func (d cluster) Description() string            { return formatDescription(api.KubernetesCluster(d)) }
func (d cluster) FilterValue() string            { return d.ClusterId }

func formatDescription(cluster api.KubernetesCluster) string {
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

func ssh(ctx context.Context, cluster *api.KubernetesCluster, args []string) error {
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
		"-p", fmt.Sprintf("%d", localPort), "root@127.0.0.1")

	cmd := exec.CommandContext(ctx, "ssh", args...)
	return localexec.RunInteractive(ctx, cmd)
}

func portForward(ctx context.Context, cluster *api.KubernetesCluster, targetPort int) error {
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

			proxyConn, err := api.DialPort(ctx, cluster, targetPort)
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

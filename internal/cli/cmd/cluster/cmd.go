// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnnet"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
)

var ErrEmptyClusteList = errors.New("no clusters")

const (
	NsGitHubAttachmentUrlType = "namespacelabs.dev/foundation/schema.GithubAttachment"
)

func NewClusterCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "cluster",
		Short:  "Cluster-related activities.",
		Hidden: hidden,
	}

	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newSshCmd())
	cmd.AddCommand(newPortForwardCmd())
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(NewKubectlCmd())
	cmd.AddCommand(newKubeconfigCmd())
	cmd.AddCommand(newBuildctlCmd())
	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newHistoryCmd())
	cmd.AddCommand(newLogsCmd())

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

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists all of your clusters.",
		Args:  cobra.NoArgs,
	}

	rawOutput := cmd.Flags().Bool("raw_output", false, "Dump the resulting server response, without formatting.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := false
		clusters, err := api.ListClusters(ctx, api.Endpoint, history)
		if err != nil {
			return err
		}

		if *rawOutput {
			stdout := console.Stdout(ctx)
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(clusters)
		}
		if len(clusters.Clusters) == 0 {
			printCreateClusterMsg(ctx)
			return nil
		}
		return staticTableClusters(ctx, clusters.Clusters, history)
	})

	return cmd
}

func newSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [cluster-id]",
		Short: "Start an SSH session.",
		Args:  cobra.MaximumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, err := selectCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusteList) {
				printCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		return dossh(ctx, cluster, nil)
	})

	return cmd
}

func newPortForwardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward [cluster-id]",
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
			if errors.Is(err, ErrEmptyClusteList) {
				printCreateClusterMsg(ctx)
				return nil
			}
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
		Use:   "destroy [cluster-id]",
		Short: "Destroys an existing cluster.",
		Args:  cobra.ArbitraryArgs,
	}

	force := cmd.Flags().Bool("force", false, "Skip the confirmation step.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusters, err := selectClusters(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusteList) {
				printCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		for _, cluster := range clusters {
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

			if err := api.DestroyCluster(ctx, api.Endpoint, cluster.ClusterId); err != nil {
				return err
			}
		}

		return nil
	})

	return cmd
}

func NewKubectlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl -- ...",
		Short: "Run kubectl on the target cluster.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusterName := args[0]
		args = args[1:]

		response, err := api.GetCluster(ctx, api.Endpoint, clusterName)
		if err != nil {
			return err
		}

		cluster := response.Cluster

		cfg, err := kubectl.WriteRawKubeconfig(ctx, ctl.MakeConfig(cluster), false)
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
		cluster, err := selectCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusteList) {
				printCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		cfg, err := kubectl.WriteRawKubeconfig(ctx, ctl.MakeConfig(cluster), false)
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

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "History of your previous running clusters.",
		Args:  cobra.NoArgs,
	}

	rawOutput := cmd.Flags().Bool("raw_output", false, "Dump the resulting server response, without formatting.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		history := true
		clusters, err := api.ListClusters(ctx, api.Endpoint, history)
		if err != nil {
			return err
		}

		if *rawOutput {
			stdout := console.Stdout(ctx)
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(clusters)
		}

		if len(clusters.Clusters) == 0 {
			printCreateClusterMsg(ctx)
			return nil
		}

		return staticTableClusters(ctx, clusters.Clusters, history)
	})

	return cmd
}

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [cluster-id]",
		Short: "Prints log for a cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	follow := cmd.Flags().BoolP("follow", "f", false, "Specify if the logs should be streamed.")
	since := cmd.Flags().Duration("since", time.Duration(0), "Show logs since a relative timestamp (e.g. 42m for 42 minutes). The flag can't be use with --follow.")
	namespace := cmd.Flags().StringP("namespace", "n", "", "Print the logs of this namespace.")
	pod := cmd.Flags().StringP("pod", "p", "", "Print the logs of this pod.")
	container := cmd.Flags().StringP("container", "c", "", "Print the logs of this container.")
	raw := cmd.Flags().Bool("raw", false, "Print the raw logs to stdout (skipping namespace/pod labels).")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, err := selectCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusteList) {
				printCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		if *follow && *since != time.Duration(0) {
			return fnerrors.New("--follow flag can't be used with --since flag")
		}

		var includeSelector []*api.LogsSelector
		if *namespace != "" || *pod != "" || *container != "" {
			includeSelector = append(includeSelector, &api.LogsSelector{
				Namespace: *namespace,
				Pod:       *pod,
				Container: *container,
			})
		}

		lp := newLogPrinter(*raw)

		if *follow {
			handle := func(lb api.LogBlock) error {
				lp.Print(ctx, lb)
				return nil
			}

			return api.TailClusterLogs(ctx, api.Endpoint, &api.LogsOpts{
				ClusterID: cluster.ClusterId,
				Include:   includeSelector,
			}, handle)
		}

		logOpts := &api.LogsOpts{
			ClusterID: cluster.ClusterId,
			Include:   includeSelector,
		}
		if *since != time.Duration(0) {
			ts := time.Now().Add(-1 * (*since))
			logOpts.StartTs = &ts
		}

		logs, err := api.GetClusterLogs(ctx, api.Endpoint, logOpts)
		if err != nil {
			return fnerrors.New("failed to get cluster logs: %w", err)
		}

		for _, lb := range logs.LogBlock {
			lp.Print(ctx, lb)
		}

		return nil
	})

	return cmd
}

func selectClusters(ctx context.Context, names []string) ([]*api.KubernetesCluster, error) {
	var res []*api.KubernetesCluster
	previousRuns := false
	for _, name := range names {
		response, err := api.GetCluster(ctx, api.Endpoint, name)
		if err != nil {
			return nil, err
		}
		res = append(res, response.Cluster)
	}

	if len(res) > 0 {
		return res, nil
	}

	clusters, err := api.ListClusters(ctx, api.Endpoint, previousRuns)
	if err != nil {
		return nil, err
	}
	if len(clusters.Clusters) == 0 {
		return nil, ErrEmptyClusteList
	}

	cl, err := selectTableClusters(ctx, clusters.Clusters, previousRuns)
	if err != nil || cl == nil {
		return nil, err
	}

	return []*api.KubernetesCluster{cl}, nil
}

func selectCluster(ctx context.Context, args []string) (*api.KubernetesCluster, error) {
	clusters, err := selectClusters(ctx, args)
	if err != nil {
		return nil, err
	}

	switch len(clusters) {
	case 1:
		return clusters[0], nil
	case 0:
		return nil, nil
	default:
		return nil, fnerrors.InternalError("got %d clusters - expected one", len(clusters))
	}
}

const (
	idColKey       = "id"
	cpuColKey      = "cpu"
	memColKey      = "mem"
	createdColKey  = "created"
	durationColKey = "duration"
	ttlColKey      = "ttl"
	purposeColKey  = "purpose"
)

func tableClusters(ctx context.Context,
	clusters []api.KubernetesCluster, previousRuns, selectCluster bool) (*api.KubernetesCluster, error) {
	clusterIdMap := map[string]api.KubernetesCluster{}
	cols := []tui.Column{
		{Key: idColKey, Title: "Cluster ID", MinWidth: 5, MaxWidth: 20},
		{Key: cpuColKey, Title: "CPU", MinWidth: 5, MaxWidth: 20},
		{Key: memColKey, Title: "Memory", MinWidth: 5, MaxWidth: 20},
		{Key: createdColKey, Title: "Created", MinWidth: 10, MaxWidth: 20},
	}
	if previousRuns {
		cols = append(cols, tui.Column{Key: durationColKey, Title: "Duration", MinWidth: 10, MaxWidth: 20})
	} else {
		cols = append(cols, tui.Column{Key: ttlColKey, Title: "Time to live", MinWidth: 5, MaxWidth: 20})
	}
	cols = append(cols, tui.Column{Key: purposeColKey, Title: "Purpose", MinWidth: 10, MaxWidth: 100})

	rows := []tui.Row{}
	for _, cluster := range clusters {
		clusterIdMap[cluster.ClusterId] = cluster
		if previousRuns && cluster.DestroyedAt == "" {
			continue
		}
		cpu := fmt.Sprintf("%d", cluster.Shape.VirtualCpu)
		ram := humanize.IBytes(uint64(cluster.Shape.MemoryMegabytes) * humanize.MiByte)
		created, _ := time.Parse(time.RFC3339, cluster.Created)
		deadline, _ := time.Parse(time.RFC3339, cluster.Deadline)

		row := tui.Row{
			idColKey:      cluster.ClusterId,
			cpuColKey:     cpu,
			memColKey:     ram,
			createdColKey: humanize.Time(created.Local()),
		}
		if previousRuns {
			destroyedAt, _ := time.Parse(time.RFC3339, cluster.DestroyedAt)
			row[durationColKey] = destroyedAt.Sub(created).Truncate(time.Second).String()
		} else {
			row[ttlColKey] = humanize.Time(deadline.Local())
		}
		row[purposeColKey] = formatPurpose(cluster)
		rows = append(rows, row)
	}

	if selectCluster {
		row, err := tui.SelectTable(ctx, cols, rows)
		if err != nil || row == nil {
			return nil, err
		}
		cl := clusterIdMap[row[0]]
		return &cl, nil
	}
	err := tui.StaticTable(ctx, cols, rows)

	return nil, err
}

func formatPurpose(cluster api.KubernetesCluster) string {
	purpose := "-"
	if len(cluster.Attachment) > 0 {
		for _, att := range cluster.Attachment {
			if att.TypeURL == NsGitHubAttachmentUrlType {
				var ghAttach schema.GitHubAttachment
				if err := json.Unmarshal(att.Content, &ghAttach); err == nil {
					purpose = fmt.Sprintf("GH Action: %s %s",
						ghAttach.Repository, ghAttach.RunId)
					break
				}
			}
		}
	} else if cluster.DocumentedPurpose != "" {
		purpose = cluster.DocumentedPurpose
	}
	return purpose
}

func staticTableClusters(ctx context.Context,
	clusters []api.KubernetesCluster, previousRuns bool) error {
	_, err := tableClusters(ctx, clusters, previousRuns, false)
	return err
}

func selectTableClusters(ctx context.Context,
	clusters []api.KubernetesCluster, previousRuns bool) (*api.KubernetesCluster, error) {
	return tableClusters(ctx, clusters, previousRuns, true)
}

type cluster api.KubernetesCluster

func (d cluster) Cluster() api.KubernetesCluster { return api.KubernetesCluster(d) }
func (d cluster) Title() string                  { return d.ClusterId }
func (d cluster) Description() string            { return formatDescription(api.KubernetesCluster(d), false) }
func (d cluster) FilterValue() string            { return d.ClusterId }

func formatDescription(cluster api.KubernetesCluster, history bool) string {
	cpu := "<unknown>"
	ram := "<unknown>"

	if shape := cluster.Shape; shape != nil {
		cpu = fmt.Sprintf("%d", shape.VirtualCpu)
		ram = humanize.IBytes(uint64(shape.MemoryMegabytes) * humanize.MiByte)
	}

	created, _ := time.Parse(time.RFC3339, cluster.Created)
	deadline, _ := time.Parse(time.RFC3339, cluster.Deadline)
	destroyedAt, _ := time.Parse(time.RFC3339, cluster.DestroyedAt)

	if history {
		return fmt.Sprintf("[cpu: %s ram: %s] (created %v, destroyed %v, lasted %v, dist: %s): %s",
			cpu, ram, created.Local(), destroyedAt.Local(), destroyedAt.Sub(created),
			cluster.KubernetesDistribution, cluster.DocumentedPurpose)
	}
	return fmt.Sprintf("[cpu: %s ram: %s] (created %v, for %v, dist: %s): %s",
		cpu, ram, created.Local(), time.Until(deadline),
		cluster.KubernetesDistribution, cluster.DocumentedPurpose)
}

func dossh(ctx context.Context, cluster *api.KubernetesCluster, args []string) error {
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

				peerConn, err := api.DialPort(ctx, cluster, 22)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
					return
				}

				go func() {
					_, _ = io.Copy(conn, peerConn)
				}()

				_, _ = io.Copy(peerConn, conn)
			}()
		}
	}()

	localPort := lst.Addr().(*net.TCPAddr).Port

	args = append(args,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "UpdateHostKeys no",
		"-p", fmt.Sprintf("%d", localPort), "root@127.0.0.1")

	if cluster.SshPrivateKey != nil {
		f, err := dirs.CreateUserTemp("ssh", "privatekey")
		if err != nil {
			return err
		}

		defer os.Remove(f.Name())

		if _, err := f.Write(cluster.SshPrivateKey); err != nil {
			return err
		}

		args = append(args, "-i", f.Name())
	}

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

func printCreateClusterMsg(ctx context.Context) {
	stdout := console.Stdout(ctx)
	fmt.Fprintf(stdout, "No clusters. Try creating one with `%s cluster create`.\n", os.Args[0])
}

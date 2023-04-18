// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

var ErrEmptyClusterList = errors.New("no clusters")

// Used by `nsc` (and as the basis to `ns cluster`)
func NewBareClusterCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "cluster",
		Short:  "Cluster-related activities.",
		Hidden: hidden,
	}

	cmd.AddCommand(NewCreateCmd(true)) // Adding hidden command under `cluster` to support old action versions.
	cmd.AddCommand(newPortForwardCmd())
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(newKubeconfigCmd())
	cmd.AddCommand(newHistoryCmd())
	cmd.AddCommand(newDockerCmd())
	cmd.AddCommand(NewProxyCmd())
	cmd.AddCommand(NewDockerLoginCmd(true)) // Adding hidden command under `cluster` to support old action versions.
	cmd.AddCommand(NewMetadataCmd())

	h := &cobra.Command{
		Use:    "internal",
		Hidden: true,
	}

	cmd.AddCommand(h)

	h.AddCommand(newReleaseCmd())
	h.AddCommand(newWakeCmd())

	return cmd
}

// Used as `ns cluster`
func NewClusterCmd(hidden bool) *cobra.Command {
	cmd := NewBareClusterCmd(hidden)

	cmd.AddCommand(NewCreateCmd(false))
	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewSshCmd())
	cmd.AddCommand(NewKubectlCmd())
	cmd.AddCommand(NewBuildCmd())
	cmd.AddCommand(NewLogsCmd())
	cmd.AddCommand(NewProxyCmd())
	cmd.AddCommand(NewRunCmd())
	cmd.AddCommand(NewRunComposeCmd())
	cmd.AddCommand(NewExposeCmd())
	cmd.AddCommand(NewBuildkitCmd())

	return cmd
}

func selectClusterID(ctx context.Context) (string, error) {
	previousRuns := false

	clusters, err := api.ListClusters(ctx, api.Endpoint, previousRuns, nil)
	if err != nil {
		return "", err
	}
	if len(clusters.Clusters) == 0 {
		return "", ErrEmptyClusterList
	}

	cl, err := selectTableClusters(ctx, clusters.Clusters, previousRuns)
	if err != nil || cl == nil {
		return "", err
	}

	return cl.ClusterId, nil
}

func selectCluster(ctx context.Context, args []string) (*api.KubernetesCluster, []string, error) {
	if len(args) > 1 {
		return nil, nil, fnerrors.InternalError("got %d clusters - expected one", len(args))
	}

	var clusterID string
	if len(args) == 1 {
		clusterID = args[0]
		args = args[1:]
	} else {
		selected, err := selectClusterID(ctx)
		if err != nil {
			return nil, nil, err
		}
		if selected == "" {
			return nil, nil, nil
		}
		clusterID = selected
	}

	response, err := api.EnsureCluster(ctx, api.Endpoint, clusterID)
	if err != nil {
		return nil, nil, err
	}

	return response.Cluster, args, nil
}

const (
	idColKey       = "id"
	cpuColKey      = "cpu"
	memColKey      = "mem"
	createdColKey  = "created"
	durationColKey = "duration"
	ttlColKey      = "ttl"
	purposeColKey  = "purpose"
	platformColKey = "platform"
)

func tableClusters(ctx context.Context,
	clusters []api.KubernetesClusterMetadata, previousRuns, selectCluster bool) (*api.KubernetesClusterMetadata, error) {
	clusterIdMap := map[string]api.KubernetesClusterMetadata{}
	cols := []tui.Column{
		{Key: idColKey, Title: "Cluster ID", MinWidth: 5, MaxWidth: 20},
		{Key: cpuColKey, Title: "CPU", MinWidth: 3, MaxWidth: 20},
		{Key: memColKey, Title: "Memory", MinWidth: 5, MaxWidth: 20},
		{Key: platformColKey, Title: "Arch", MinWidth: 5, MaxWidth: 10},
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
		arch := strings.ToLower(cluster.Shape.MachineArch)
		created, _ := time.Parse(time.RFC3339, cluster.Created)
		deadline, _ := time.Parse(time.RFC3339, cluster.Deadline)

		row := tui.Row{
			idColKey:       cluster.ClusterId,
			cpuColKey:      cpu,
			memColKey:      ram,
			platformColKey: arch,

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

func formatPurpose(cluster api.KubernetesClusterMetadata) string {
	purpose := "-"
	if cluster.GithubWorkflow != nil {
		purpose = fmt.Sprintf("GH Action: %s %s",
			cluster.GithubWorkflow.Repository, cluster.GithubWorkflow.RunId)
	} else if cluster.DocumentedPurpose != "" {
		purpose = cluster.DocumentedPurpose
	}
	return purpose
}

func staticTableClusters(ctx context.Context,
	clusters []api.KubernetesClusterMetadata, previousRuns bool) error {
	_, err := tableClusters(ctx, clusters, previousRuns, false)
	return err
}

func selectTableClusters(ctx context.Context,
	clusters []api.KubernetesClusterMetadata, previousRuns bool) (*api.KubernetesClusterMetadata, error) {
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

func printCreateClusterMsg(ctx context.Context) {
	stdout := console.Stdout(ctx)
	fmt.Fprintf(stdout, "No clusters. Try creating one with `%s cluster create`.\n", os.Args[0])
}

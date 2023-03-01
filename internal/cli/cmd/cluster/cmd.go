// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

var ErrEmptyClusteList = errors.New("no clusters")

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
	if cluster.GithubWorkflow != nil {
		purpose = fmt.Sprintf("GH Action: %s %s",
			cluster.GithubWorkflow.Repository, cluster.GithubWorkflow.RunId)
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

func printCreateClusterMsg(ctx context.Context) {
	stdout := console.Stdout(ctx)
	fmt.Fprintf(stdout, "No clusters. Try creating one with `%s cluster create`.\n", os.Args[0])
}

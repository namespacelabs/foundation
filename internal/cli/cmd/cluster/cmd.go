// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"namespacelabs.dev/foundation/internal/cli/fncobra/name"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

var ErrEmptyClusterList = errors.New("no instances")

// Used by `nsc` (and as the basis to `ns cluster`)
func NewBareClusterCmd(use string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    use,
		Short:  "Instance-related activities.",
		Hidden: hidden,
	}

	cmd.AddCommand(NewCreateCmd())
	cmd.AddCommand(newPortForwardCmd())
	cmd.AddCommand(NewDestroyCmd())
	cmd.AddCommand(newWriteKubeconfigCmd("kubeconfig", true)) // Adding hidden command under `cluster` to support old action versions.
	cmd.AddCommand(newHistoryCmd())
	cmd.AddCommand(newDockerPassthroughCmd())
	cmd.AddCommand(NewProxyCmd())
	cmd.AddCommand(newDockerLoginCmd(true)) // Adding hidden command under `cluster` to support old action versions.
	cmd.AddCommand(NewMetadataCmd())
	cmd.AddCommand(NewExtendDurationCmd("extend-duration"))

	h := &cobra.Command{
		Use:    "internal",
		Hidden: true,
	}

	cmd.AddCommand(h)

	h.AddCommand(newSuspendCmd())
	h.AddCommand(newReleaseCmd())
	h.AddCommand(newWakeCmd())

	return cmd
}

// Used as `ns cluster`
func NewClusterCmd(hidden bool) *cobra.Command {
	cmd := NewBareClusterCmd("cluster", hidden)

	cmd.AddCommand(NewCreateCmd())
	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewSshCmd())
	cmd.AddCommand(NewVncCmd())
	cmd.AddCommand(NewRdpCmd())
	cmd.AddCommand(NewKubectlCmd())
	cmd.AddCommand(NewBuildCmd())
	cmd.AddCommand(NewLogsCmd())
	cmd.AddCommand(NewProxyCmd())
	cmd.AddCommand(NewRunCmd())
	cmd.AddCommand(NewExposeCmd())
	cmd.AddCommand(NewBuildkitCmd())

	return cmd
}

func selectClusterID(ctx context.Context, previousRuns bool) (string, error) {
	clusters, err := api.ListClusters(ctx, api.Methods, api.ListOpts{PreviousRuns: previousRuns})
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

func SelectRunningCluster(ctx context.Context, args []string) (*api.KubernetesCluster, []string, error) {
	var clusterID string
	if len(args) >= 1 {
		clusterID = args[0]
		args = args[1:]
	} else {
		selected, err := selectClusterID(ctx, false /* previousRuns */)
		if err != nil {
			return nil, nil, err
		}
		if selected == "" {
			return nil, nil, nil
		}
		clusterID = selected
	}

	response, err := api.EnsureCluster(ctx, api.Methods, nil, clusterID)
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
	osColKey       = "os"
)

func tableClusters(ctx context.Context,
	clusters []api.KubernetesClusterMetadata, previousRuns, selectCluster bool) (*api.KubernetesClusterMetadata, error) {
	clusterIdMap := map[string]api.KubernetesClusterMetadata{}
	cols := []tui.Column{
		{Key: idColKey, Title: "Instance ID", MinWidth: 5, MaxWidth: 20},
		{Key: cpuColKey, Title: "CPU", MinWidth: 3, MaxWidth: 20},
		{Key: memColKey, Title: "Memory", MinWidth: 5, MaxWidth: 20},
		{Key: osColKey, Title: "OS", MinWidth: 5, MaxWidth: 10},
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
		os := formatOS(cluster.Shape)
		arch := strings.ToLower(cluster.Shape.MachineArch)
		created, _ := time.Parse(time.RFC3339, cluster.Created)
		deadline, _ := time.Parse(time.RFC3339, cluster.Deadline)

		row := tui.Row{
			idColKey:       cluster.ClusterId,
			cpuColKey:      cpu,
			memColKey:      ram,
			osColKey:       os,
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

func formatOS(shape *api.ClusterShape) string {
	switch shape.OS {
	case "macos":
		return "MacOS"
	default:
		return cases.Title(language.English).String(shape.OS)
	}
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

func PrintCreateClusterMsg(ctx context.Context) {
	stdout := console.Stdout(ctx)

	var cmd string
	switch name.CmdName {
	case "nsc":
		cmd = "nsc create"
	default:
		cmd = fmt.Sprintf("%s cluster create", name.CmdName)
	}

	fmt.Fprintf(stdout, "No instances. Try creating one with `%s`.\n", cmd)
}

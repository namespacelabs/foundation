// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
)

func NewLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [cluster-id]",
		Short: "Prints logs for a cluster.",
		Long:  "Prints application logs for a cluster. To print all cluster logs (including Kubernetes system logs) add --all.",
		Args:  cobra.MaximumNArgs(1),
	}

	follow := cmd.Flags().BoolP("follow", "f", false, "Specify if logs should be streamed continuously.")
	since := cmd.Flags().Duration("since", time.Duration(0), "Show logs relative to a timestamp (e.g. 42m for 42 minutes). The flag can't be use with --follow.")
	namespace := cmd.Flags().StringP("namespace", "n", "", "If specified, only display logs of this Kubernetes namespace.")
	pod := cmd.Flags().StringP("pod", "p", "", "If specified, only display logs of this Kubernetes Pod.")
	container := cmd.Flags().StringP("container", "c", "", "If specified, only display logs of this container.")
	source := cmd.Flags().StringP("source", "s", "kubernetes", "If specified, display logs from this source. Default: kubernetes")
	raw := cmd.Flags().Bool("raw", false, "Output raw logs (skipping namespace/pod labels).")
	all := cmd.Flags().Bool("all", false, "Output all logs (including Kubernetes system logs).")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		var clusterID string
		if len(args) == 1 {
			clusterID = args[0]
		} else {
			var err error
			clusterID, err = selectClusterID(ctx, true /* previousRuns */)
			if err != nil {
				if errors.Is(err, ErrEmptyClusterList) {
					PrintCreateClusterMsg(ctx)
					return nil
				}
				return err
			}
		}

		if clusterID == "" {
			return nil
		}

		if *follow && *since != time.Duration(0) {
			return fnerrors.New("--follow flag can't be used with --since flag")
		}

		var includeSelector []*api.LogsSelector
		var excludeSelector []*api.LogsSelector
		if *namespace != "" || *pod != "" || *container != "" {
			sel := &api.LogsSelector{
				Source:    *source,
				Namespace: *namespace,
			}

			switch *source {
			case "kubernetes":
				sel.ContainerName = *container
				sel.PodName = *pod
			case "containerd":
				if *pod != "" {
					return fnerrors.New("--pod flag can't be used with source 'containerd'")
				}
				sel.ContainerID = *container
			default:
				return fnerrors.New("unsupported logs source %q, only 'containerd' and 'kubernetes' are supported", *source)
			}

			includeSelector = append(includeSelector, sel)
		} else if !*all {
			for _, ns := range ctl.SystemNamespaces {
				excludeSelector = append(excludeSelector, &api.LogsSelector{
					Source:    *source,
					Namespace: ns,
				})
			}
		}

		lp := newLogPrinter(*raw)

		if *follow {
			handle := func(lb api.LogBlock) error {
				lp.Print(ctx, lb)
				return nil
			}

			return api.TailClusterLogs(ctx, api.Methods, &api.LogsOpts{
				ClusterID: clusterID,
				Include:   includeSelector,
				Exclude:   excludeSelector,
			}, handle)
		}

		logOpts := &api.LogsOpts{
			ClusterID: clusterID,
			Include:   includeSelector,
			Exclude:   excludeSelector,
		}
		if *since != time.Duration(0) {
			ts := time.Now().Add(-1 * (*since))
			logOpts.StartTs = &ts
		}

		cluster, err := api.GetCluster(ctx, api.Methods, clusterID)
		if err != nil {
			return fnerrors.New("failed to get cluster information: %w", err)
		}

		if cluster.Cluster != nil {
			logOpts.IngressDomain = cluster.Cluster.IngressDomain
		}

		logs, err := api.GetClusterLogs(ctx, api.Methods, logOpts)
		if err != nil {
			return fnerrors.New("failed to get cluster logs: %w", err)
		}

		// Skip hint when running in raw mode.
		if !*raw && len(logs.LogBlock) == 0 {
			fmt.Fprintf(console.Stdout(ctx), "No logs found.\n")

			style := colors.Ctx(ctx)
			if *source == "kubernetes" {
				fmt.Fprintf(console.Stdout(ctx),
					"\n  Try running %s to also fetch Kubernetes system logs.\n",
					style.Highlight.Apply(fmt.Sprintf("nsc logs %s --all", clusterID)),
				)
			}

			return nil
		}

		for _, lb := range logs.LogBlock {
			lp.Print(ctx, lb)
		}

		return nil
	})

	return cmd
}

const (
	namespaceLogLabel  = "namespace"
	k8sPodNameLogLabel = "kubernetes_pod_name"
)

var defaultNamespaces = []string{"", "default"}

type logPrinter struct {
	outs      map[string]io.Writer
	useStdout bool
}

func newLogPrinter(useStdout bool) *logPrinter {
	return &logPrinter{
		outs:      make(map[string]io.Writer),
		useStdout: useStdout,
	}
}

func (lp *logPrinter) writer(ctx context.Context, labels map[string]string, stream string) io.Writer {
	if lp.useStdout {
		return console.Stdout(ctx)
	}

	// Cache writers so that we get the same color for each output on the same stream
	keys := maps.Keys(labels)
	sort.Strings(keys)

	key := stream
	for _, k := range keys {
		key = fmt.Sprintf("%s/%s:%s", key, k, labels[k])
	}

	if out, ok := lp.outs[key]; ok {
		return out
	}

	// Only use namespace and pod name in user visible label since console space is limited
	var label string
	if pod, ok := labels[k8sPodNameLogLabel]; ok {
		label = pod

		if ns, ok := labels[namespaceLogLabel]; ok && !slices.Contains(defaultNamespaces, ns) {
			label = fmt.Sprintf("%s/%s", ns, label)
		}
	} else {
		label = stream
	}

	out := console.Output(ctx, label)
	lp.outs[key] = out
	return out
}

func (lp *logPrinter) Print(ctx context.Context, lb api.LogBlock) {
	for _, l := range lb.Line {
		out := lp.writer(ctx, lb.Labels, l.Stream)
		fmt.Fprintf(out, "%s %s\n", l.Timestamp.Format(time.RFC3339), l.Content)
	}
}

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
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/timestamppb"

	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [instance-id]",
		Short: "Prints logs for a instance.",
		Long:  "Prints application logs for a instance. To print all instance logs (including Kubernetes system logs) add --all.",
		Args:  cobra.MaximumNArgs(1),
	}

	follow := cmd.Flags().BoolP("follow", "f", false, "Specify if logs should be streamed continuously.")
	since := fncobra.Duration(cmd.Flags(), "since", 0, "Show logs relative to a timestamp (e.g. 42m for 42 minutes). The flag can't be use with --follow.")
	after := cmd.Flags().String("after", "", "Only show logs after this timestamp (RFC3339, e.g. 2024-01-15T10:30:00Z).")
	before := cmd.Flags().String("before", "", "Only show logs before this timestamp (RFC3339, e.g. 2024-01-15T12:00:00Z).")
	limit := cmd.Flags().Int32("limit", 1000, "Maximum number of log lines to return.")
	namespace := cmd.Flags().StringP("namespace", "n", "", "If specified, only display logs of this Kubernetes namespace.")
	pod := cmd.Flags().StringP("pod", "p", "", "If specified, only display logs of this Kubernetes Pod.")
	container := cmd.Flags().StringP("container", "c", "", "If specified, only display logs of this container.")
	source := cmd.Flags().StringP("source", "s", "kubernetes", "If specified, display logs from this source. Default: kubernetes")
	kind := cmd.Flags().StringSliceP("kind", "k", nil, "If specified, only display logs of these kinds (e.g. kubernetes, containers, system, applications). Can be specified multiple times or comma-separated.")
	raw := cmd.Flags().Bool("raw", false, "Output raw logs (skipping namespace/pod labels).")
	all := cmd.Flags().Bool("all", false, "Output all logs (including Kubernetes system logs).")
	output := cmd.Flags().StringP("output", "o", "plain", "Output format. Supported values: plain, json (outputs one JSON object per line).")

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

		if *follow && (*since != 0 || *after != "" || *before != "") {
			return fnerrors.Newf("--follow flag can't be used with --since, --after, or --before flags")
		}

		if *since != 0 && (*after != "" || *before != "") {
			return fnerrors.Newf("--since flag can't be used with --after or --before flags")
		}

		var lp logOutput
		switch *output {
		case "json":
			lp = newJSONLogPrinter()
		case "plain":
			lp = newLogPrinter(*raw)
		default:
			return fnerrors.Newf("unsupported output format %q, supported values: plain, json", *output)
		}

		if *all {
			fmt.Fprintf(console.Stderr(ctx), "Warning: --all is deprecated and has no effect.\n")
		}

		var includeSelector []*api.LogsSelector
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
					return fnerrors.Newf("--pod flag can't be used with source 'containerd'")
				}
				sel.ContainerID = *container
			default:
				return fnerrors.Newf("unsupported logs source %q, only 'containerd' and 'kubernetes' are supported", *source)
			}

			includeSelector = append(includeSelector, sel)
		}

		if *follow {
			logOpts := &api.LogsOpts{
				ClusterID: clusterID,
				Include:   includeSelector,
			}

			cluster, err := api.GetCluster(ctx, api.Methods, clusterID)
			if err != nil {
				return fnerrors.Newf("failed to get instance information: %w", err)
			}

			if cluster.Cluster != nil {
				logOpts.ApiEndpoint = cluster.Cluster.ApiEndpoint
			}

			handle := func(lb api.LogBlock) error {
				return lp.PrintBlock(ctx, lb)
			}

			return api.TailClusterLogs(ctx, api.Methods, logOpts, handle)
		}

		// Use FetchLogs (LoggingService2) for non-streaming log retrieval.
		// FORWARD direction returns lines in chronological order, allowing
		// us to print each page immediately without buffering.
		req := api.FetchLogsRequest{
			MatchInstanceIds: &api.StringMatcher{
				Op:     1, // IS_ANY_OF
				Values: []string{clusterID},
			},
			LinesPerPage: *limit,
			Direction:    1, // FORWARD
		}

		if *since != 0 {
			ts := time.Now().In(time.UTC).Add(-1 * *since)
			req.TimestampRange = &api.TimestampRange{
				After: timestamppb.New(ts),
			}
		}

		if *after != "" || *before != "" {
			if req.TimestampRange == nil {
				req.TimestampRange = &api.TimestampRange{}
			}
			if *after != "" {
				t, err := time.Parse(time.RFC3339, *after)
				if err != nil {
					return fnerrors.Newf("invalid --after timestamp: %w", err)
				}
				req.TimestampRange.After = timestamppb.New(t)
			}
			if *before != "" {
				t, err := time.Parse(time.RFC3339, *before)
				if err != nil {
					return fnerrors.Newf("invalid --before timestamp: %w", err)
				}
				req.TimestampRange.Before = timestamppb.New(t)
			}
		}

		if len(*kind) > 0 {
			req.MatchKind = &api.StringMatcher{Op: 1, Values: *kind}
		}

		// Build matchers from flags.
		if *namespace != "" || *pod != "" || *container != "" {
			switch *source {
			case "kubernetes":
				km := &api.KubernetesMatcher{}
				if *namespace != "" {
					km.MatchNamespace = &api.StringMatcher{Op: 1, Values: []string{*namespace}}
				}
				if *pod != "" {
					km.MatchPodName = &api.StringMatcher{Op: 1, Values: []string{*pod}}
				}
				if *container != "" {
					km.MatchContainerName = &api.StringMatcher{Op: 1, Values: []string{*container}}
				}
				req.KubernetesMatcher = km
			case "containerd":
				if *container != "" {
					req.ContainerMatcher = &api.ContainerMatcher{
						MatchContainerID: &api.StringMatcher{Op: 1, Values: []string{*container}},
					}
				}
			}
		}

		var totalLines int32
		for {
			logs, err := api.FetchClusterLogs(ctx, api.Methods, req)
			if err != nil {
				return fnerrors.Newf("failed to get instance logs: %w", err)
			}

			if len(logs.LogLine) == 0 {
				break
			}

			for _, l := range logs.LogLine {
				if err := lp.PrintLine(ctx, l); err != nil {
					return err
				}
			}

			totalLines += int32(len(logs.LogLine))

			if totalLines >= *limit || len(logs.PaginationCursor) == 0 {
				break
			}

			req.PaginationCursor = logs.PaginationCursor
		}

		if !*raw && totalLines == 0 {
			fmt.Fprintf(console.Stdout(ctx), "No logs found.\n")
		}

		return nil
	})

	return cmd
}

type logOutput interface {
	PrintBlock(ctx context.Context, lb api.LogBlock) error
	PrintLine(ctx context.Context, l api.LogLine) error
}

// plainLogPrinter prints logs in human-readable format with colored labels.

const (
	namespaceLogLabel  = "namespace"
	k8sPodNameLogLabel = "kubernetes_pod_name"
	systemLogLabel     = "system"
)

var defaultNamespaces = []string{"", "default"}

type plainLogPrinter struct {
	outs      map[string]io.Writer
	useStdout bool
}

func newLogPrinter(useStdout bool) *plainLogPrinter {
	return &plainLogPrinter{
		outs:      make(map[string]io.Writer),
		useStdout: useStdout,
	}
}

func visibleLogLabel(labels map[string]string, stream, source string) string {
	if pod, ok := labels[k8sPodNameLogLabel]; ok && pod != "" {
		label := pod

		if ns, ok := labels[namespaceLogLabel]; ok && !slices.Contains(defaultNamespaces, ns) {
			label = fmt.Sprintf("%s/%s", ns, label)
		}

		return label
	}

	if stream != "" {
		return stream
	}

	if system, ok := labels[systemLogLabel]; ok && system != "" {
		return system
	}

	return source
}

func (lp *plainLogPrinter) writer(ctx context.Context, labels map[string]string, stream, source string) io.Writer {
	if lp.useStdout {
		return console.Stdout(ctx)
	}

	// Cache writers so that we get the same color for each output on the same stream
	keys := maps.Keys(labels)
	sort.Strings(keys)

	key := fmt.Sprintf("%s/%s", source, stream)
	for _, k := range keys {
		key = fmt.Sprintf("%s/%s:%s", key, k, labels[k])
	}

	if out, ok := lp.outs[key]; ok {
		return out
	}

	label := visibleLogLabel(labels, stream, source)
	if label == "" {
		out := console.Stdout(ctx)
		lp.outs[key] = out
		return out
	}

	out := console.Output(ctx, label)
	lp.outs[key] = out
	return out
}

func (lp *plainLogPrinter) PrintBlock(ctx context.Context, lb api.LogBlock) error {
	for _, l := range lb.Line {
		out := lp.writer(ctx, lb.Labels, l.Stream, l.Source)
		printLogContent(out, l.Timestamp, l.Content)
	}
	return nil
}

func (lp *plainLogPrinter) PrintLine(ctx context.Context, l api.LogLine) error {
	out := lp.writer(ctx, l.Labels, l.Stream, l.Source)
	printLogContent(out, l.Timestamp, l.Content)
	return nil
}

func printLogContent(out io.Writer, ts time.Time, content string) {
	for _, line := range logicalLogLines(content) {
		fmt.Fprintf(out, "%s %s\n", ts.Format(time.RFC3339), line)
	}
}

func logicalLogLines(content string) []string {
	if content == "" {
		return []string{""}
	}

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	parts := strings.Split(normalized, "\n")
	for len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	return parts
}

// jsonLogPrinter outputs each log line as a JSON object (JSONL format).

type jsonLogPrinter struct {
	enc *json.Encoder
}

func newJSONLogPrinter() *jsonLogPrinter {
	return &jsonLogPrinter{enc: json.NewEncoder(os.Stdout)}
}

func (jp *jsonLogPrinter) PrintBlock(ctx context.Context, lb api.LogBlock) error {
	for _, l := range lb.Line {
		if err := jp.enc.Encode(l); err != nil {
			return err
		}
	}
	return nil
}

func (jp *jsonLogPrinter) PrintLine(_ context.Context, l api.LogLine) error {
	return jp.enc.Encode(l)
}

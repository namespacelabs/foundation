// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

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
		cluster, _, err := selectCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusterList) {
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

func (lp *logPrinter) writer(ctx context.Context, ns, pod, container, stream string) io.Writer {
	if lp.useStdout {
		return console.Stdout(ctx)
	}

	// Cache writers so that we get the same color for each output on the same stream
	label := fmt.Sprintf("%s/%s/%s/%s", ns, pod, container, stream)
	if out, ok := lp.outs[label]; ok {
		return out
	}

	// Only use namespace and pod name in user visible label since console space is limited
	out := console.Output(ctx, fmt.Sprintf("%s/%s", ns, pod))
	lp.outs[label] = out
	return out
}

func (lp *logPrinter) Print(ctx context.Context, lb api.LogBlock) {
	for _, l := range lb.Line {
		out := lp.writer(ctx, lb.Namespace, lb.Pod, lb.Container, l.Stream)
		fmt.Fprintf(out, "%s %s\n", l.Timestamp.Format(time.RFC3339), l.Content)
	}
}

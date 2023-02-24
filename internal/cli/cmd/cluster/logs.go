// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"io"
	"time"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

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

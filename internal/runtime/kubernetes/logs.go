// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

func (r *Cluster) FetchLogsTo(ctx context.Context, reference *runtimepb.ContainerReference, opts runtime.FetchLogsOpts, callback func(runtime.ContainerLogLine)) error {
	cpr := &kubedef.ContainerPodReference{}
	if err := reference.Opaque.UnmarshalTo(cpr); err != nil {
		return fnerrors.InternalError("invalid reference: %w", err)
	}

	return fetchPodLogs(ctx, r.cli, cpr.Namespace, cpr.PodName, cpr.Container, opts, callback)
}

func fetchPodLogs(ctx context.Context, cli *kubernetes.Clientset, namespace, podName, containerName string, opts runtime.FetchLogsOpts, callback func(runtime.ContainerLogLine)) error {
	if opts.FetchLastFailure && opts.Follow {
		return fnerrors.InternalError("can't follow logs of previous failure")
	}

	logOpts := &corev1.PodLogOptions{
		Follow:     opts.Follow,
		Container:  containerName,
		Previous:   opts.FetchLastFailure,
		Timestamps: true,
	}

	if opts.TailLines > 0 {
		var tailLines int64 = int64(opts.TailLines)
		logOpts.TailLines = &tailLines
	}

	var buf bytes.Buffer
	chunk := make([]byte, 4096)

	for {
		logsReq := cli.CoreV1().Pods(namespace).GetLogs(podName, logOpts)

		content, err := logsReq.Stream(ctx)
		if err != nil {
			return err
		}

		callback(runtime.ContainerLogLine{
			Timestamp: time.Now(),
			Event:     runtime.ContainerLogLineEvent_Connected,
		})

		defer content.Close()

		for {
			n, err := content.Read(chunk)
			if n > 0 {
				buf.Write(chunk[:n])

				for {
					if i := bytes.IndexByte(buf.Bytes(), '\n'); i >= 0 {
						lineb := makeOrReuse(chunk, i+1) // Read the \n as well.
						_, _ = buf.Read(lineb)

						line := dropCR(lineb[0:i]) // Drop the \n and the \r.

						ev := runtime.ContainerLogLine{
							Event: runtime.ContainerLogLineEvent_LogLine,
						}

						// Look for the timestamp.
						if k := bytes.IndexByte(line, ' '); k > 0 {
							if ts, err := time.Parse(time.RFC3339Nano, string(line[:k])); err == nil {
								ev.LogLine = line[k+1:]
								ev.Timestamp = ts
							}
						}

						if ev.LogLine == nil {
							ev.LogLine = line
							ev.MissingTimestamp = true
						}

						callback(ev)
					} else {
						break
					}
				}
			}

			if err == io.EOF || errors.Is(err, context.Canceled) {
				return nil
			} else if err != nil {
				if !logOpts.Follow {
					return fnerrors.InternalError("log streaming failed: %w", err)
				}

				// Got an unexpected error, lets try to resume.

				callback(runtime.ContainerLogLine{
					Timestamp: time.Now(),
					Event:     runtime.ContainerLogLineEvent_Resuming,
					ResumeErr: err,
				})

				break
			}
		}

		// When resuming, don't re-tail.
		logOpts.TailLines = nil
	}
}

func makeOrReuse(buf []byte, n int) []byte {
	if len(buf) >= n {
		return buf[:n]
	}
	return make([]byte, n)
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

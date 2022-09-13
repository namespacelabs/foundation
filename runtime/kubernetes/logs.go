// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

func (r *Cluster) FetchLogsTo(ctx context.Context, w io.Writer, reference *runtime.ContainerReference, opts runtime.FetchLogsOpts) error {
	cpr := &kubedef.ContainerPodReference{}
	if err := reference.Opaque.UnmarshalTo(cpr); err != nil {
		return fnerrors.InternalError("invalid reference: %w", err)
	}

	return fetchPodLogs(ctx, r.cli, w, cpr.Namespace, cpr.PodName, cpr.Container, opts)
}

func fetchPodLogs(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, namespace, podName, containerName string, opts runtime.FetchLogsOpts) error {
	logOpts := &corev1.PodLogOptions{Follow: opts.Follow, Container: containerName, Previous: opts.FetchLastFailure, Timestamps: opts.IncludeTimestamps}

	if opts.TailLines > 0 {
		var tailLines int64 = int64(opts.TailLines)
		logOpts.TailLines = &tailLines
	}

	logsReq := cli.CoreV1().Pods(namespace).GetLogs(podName, logOpts)

	content, err := logsReq.Stream(ctx)
	if err != nil {
		return err
	}

	defer content.Close()

	_, err = io.Copy(w, content)
	return err
}

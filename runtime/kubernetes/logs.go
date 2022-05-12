// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func (r boundEnv) fetchLogs(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, server *schema.Server, opts runtime.StreamLogsOpts) error {
	pod, err := r.resolvePod(ctx, cli, w, server)
	if err != nil {
		return err
	}

	return fetchPodLogs(ctx, cli, w, r.ns(schema.PackageName(server.PackageName)), pod.Name, "", opts)
}

func fetchPodLogs(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, namespace, podName, containerName string, opts runtime.StreamLogsOpts) error {
	logOpts := &corev1.PodLogOptions{Follow: opts.Follow, Container: containerName, Previous: opts.FetchLastFailure}

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

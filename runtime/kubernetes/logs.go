// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func (r boundEnv) fetchLogs(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, server *schema.Server, opts runtime.StreamLogsOpts) error {
	parts := strings.SplitN(opts.InstanceID, ":", 2)
	podName := parts[0]

	pod, err := r.resolvePod(ctx, cli, w, server, podName)
	if err != nil {
		return err
	}

	var containerName string
	if len(parts) > 1 {
		containerName = parts[1]
	}

	return r.fetchPodLogs(ctx, cli, w, pod.Name, containerName, opts)
}

func (r boundEnv) fetchPodLogs(ctx context.Context, cli *kubernetes.Clientset, w io.Writer, podName, containerName string, opts runtime.StreamLogsOpts) error {
	logOpts := &corev1.PodLogOptions{Follow: opts.Follow, Container: containerName}

	if opts.TailLines > 0 {
		var tailLines int64 = int64(opts.TailLines)
		logOpts.TailLines = &tailLines
	}

	logsReq := cli.CoreV1().Pods(r.ns()).GetLogs(podName, logOpts)

	content, err := logsReq.Stream(ctx)
	if err != nil {
		return err
	}

	defer content.Close()

	_, err = io.Copy(w, content)
	return err
}

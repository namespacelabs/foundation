// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"os"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8srt "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

func (r K8sRuntime) startTerminal(ctx context.Context, cli *kubernetes.Clientset, server *schema.Server, rio runtime.TerminalIO, cmd []string) error {
	pod, err := r.resolvePod(ctx, cli, rio.Stderr, server)
	if err != nil {
		return err
	}

	return r.lowLevelAttachTerm(ctx, cli, pod.Namespace, pod.Name, rio, "exec", &corev1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     rio.TTY,
	})
}

func (r Unbound) attachTerminal(ctx context.Context, cli *kubernetes.Clientset, opaque *kubedef.ContainerPodReference, rio runtime.TerminalIO) error {
	return r.lowLevelAttachTerm(ctx, cli, opaque.Namespace, opaque.PodName, rio, "attach", &corev1.PodAttachOptions{
		Container: opaque.Container,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       rio.TTY,
	})
}

func (r Unbound) lowLevelAttachTerm(ctx context.Context, cli *kubernetes.Clientset, ns, podname string, rio runtime.TerminalIO, subresource string, params k8srt.Object) error {
	config, err := resolveConfig(ctx, r.host)
	if err != nil {
		return err
	}

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return err
	}

	req := restClient.Post().
		Resource("pods").
		Name(podname).
		Namespace(ns).
		SubResource(subresource).
		VersionedParams(params, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fnerrors.InvocationError("creating executor failed: %w", err)
	}

	opts := remotecommand.StreamOptions{
		Stdin:             rio.Stdin,
		Stdout:            rio.Stdout,
		Stderr:            rio.Stderr,
		Tty:               rio.TTY,
		TerminalSizeQueue: nil, // Set below.
	}

	if rio.ResizeQueue != nil {
		opts.TerminalSizeQueue = readResizeQueue{rio.ResizeQueue}
	}

	// XXX move this check somewhere else.
	if rio.Stdin == os.Stdin {
		restore, err := termios.MakeRaw(os.Stdin.Fd())
		if err != nil {
			return err
		}

		defer func() {
			_ = restore()
		}()

		done := console.EnterInputMode(ctx, "Press enter, if you don't see any output.\n")
		defer done()
	}

	if err := exec.Stream(opts); err != nil {
		if s, ok := err.(*k8serrors.StatusError); ok {
			return fnerrors.InvocationError("%+v: failed to attach terminal: %w", s.ErrStatus, err)
		}

		return fnerrors.InvocationError("stream failed: %w", err)
	}

	return nil
}

type readResizeQueue struct{ ch chan termios.WinSize }

func (r readResizeQueue) Next() *remotecommand.TerminalSize {
	v, ok := <-r.ch
	if !ok {
		return nil // Channel closed.
	}
	return &remotecommand.TerminalSize{Width: v.Width, Height: v.Height}
}

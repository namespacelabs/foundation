// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"io"
	"net/http"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"namespacelabs.dev/foundation/internal/console"
)

func (r boundEnv) startAndBlockPortFwd(ctx context.Context, ns, podName string, localAddrs, containerPorts []string, stopCh chan struct{}, reportPorts func([]portforward.ForwardedPort)) error {
	config, err := r.makeDefaultConfig()
	if err != nil {
		return err
	}

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return err
	}

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return err
	}

	req := restClient.Post().
		Resource("pods").
		Namespace(ns).
		Name(podName).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	readyChannel := make(chan struct{}) // Used internally by portforward to signal we're actually listening.

	out := console.Output(ctx, "portfwderr")

	pfw, err := portforward.NewOnAddresses(dialer, localAddrs, containerPorts, stopCh, readyChannel, io.Discard, out)
	if err != nil {
		return err
	}

	if reportPorts != nil {
		go func() {
			select {
			case <-ctx.Done():
				// Cancelled before we had a chance to report port.
			case <-readyChannel:
				ports, _ := pfw.GetPorts()
				reportPorts(ports)
			}
		}()
	}

	return pfw.ForwardPorts()
}
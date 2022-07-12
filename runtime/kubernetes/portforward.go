// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport/spdy"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnnet"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/schema"
)

type PodResolver interface {
	Resolve(context.Context) (v1.Pod, error)
}

type StartAndBlockPortFwdArgs struct {
	Namespace     string
	Identifier    string
	LocalAddrs    []string
	LocalPort     int // 0 to be dynamically allocated.
	ContainerPort int
	PodResolver   PodResolver
	ReportPorts   func(runtime.ForwardedPort)
}

const PortForwardProtocolV1Name = "portforward.k8s.io"

func (r K8sRuntime) ForwardPort(ctx context.Context, server *schema.Server, containerPort int32, localAddrs []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
	if containerPort <= 0 {
		return nil, fnerrors.UserError(server, "invalid port number: %d", containerPort)
	}

	ns := serverNamespace(r, server)

	return r.RawForwardPort(ctx, server.PackageName, ns, map[string]string{kubedef.K8sServerId: server.Id}, int(containerPort), localAddrs, callback)
}

func (u Unbound) RawForwardPort(ctx context.Context, desc, ns string, podLabels map[string]string, containerPort int, localAddrs []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	p := kubeobserver.NewPodObserver(ctxWithCancel, u.cli, ns, podLabels)

	go func() {
		if err := u.StartAndBlockPortFwd(ctxWithCancel, StartAndBlockPortFwdArgs{
			Namespace:     ns,
			Identifier:    desc,
			LocalAddrs:    localAddrs,
			LocalPort:     0,
			ContainerPort: containerPort,
			PodResolver:   p,
			ReportPorts:   callback,
		}); err != nil {
			fmt.Fprintf(console.Errors(ctx), "port forwarding for %s (%d) failed: %v\n", desc, containerPort, err)
		}
	}()

	return closerCallback(cancel), nil
}

func (r Unbound) StartAndBlockPortFwd(ctx context.Context, args StartAndBlockPortFwdArgs) error {
	config, err := resolveConfig(ctx, r.host)
	if err != nil {
		return err
	}

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return err
	}

	debug := console.Debug(ctx)

	eg := executor.New(ctx, "kubernetes.portforward")

	for _, localAddr := range args.LocalAddrs {
		lst, err := fnnet.ListenPort(ctx, localAddr, args.LocalPort, args.ContainerPort)
		if err != nil {
			return err
		}

		eg.Go(func(ctx context.Context) error {
			defer lst.Close()

			localPort := lst.Addr().(*net.TCPAddr).Port
			fmt.Fprintf(debug, "kube/portfwd: %s: %d: listening on port %d\n", args.Identifier, args.ContainerPort, localPort)

			args.ReportPorts(runtime.ForwardedPort{
				LocalPort:     uint(localPort),
				ContainerPort: uint(args.ContainerPort),
			})

			for {
				conn, err := lst.Accept()
				if err != nil {
					return err
				}

				fmt.Fprintf(debug, "kube/portfwd: %s: %d: accepted new connection: %v\n", args.Identifier, args.ContainerPort, conn.RemoteAddr())

				eg.Go(func(ctx context.Context) error {
					defer conn.Close()

					t := time.Now()
					pod, err := args.PodResolver.Resolve(ctx)
					if err != nil {
						fmt.Fprintf(debug, "kube/portfwd: %s: %d: %s: resolve failed: %v\n", args.Identifier, args.ContainerPort, conn.RemoteAddr(), err)
						return nil // Do not propagate error.
					}

					fmt.Fprintf(debug, "kube/portfwd: %s: %d: %s: resolved to %s/%s (took %v)\n", args.Identifier, args.ContainerPort, conn.RemoteAddr(), pod.Namespace, pod.Name, time.Since(t))

					t = time.Now()
					streamConn, err := dialPod(ctx, restClient, config, pod.Namespace, pod.Name)
					if err != nil {
						fmt.Fprintf(debug, "kube/portfwd: %s: %d: %s: stream connect failed: %v\n", args.Identifier, args.ContainerPort, conn.RemoteAddr(), err)
						return nil // Do not propagate error.
					}

					fmt.Fprintf(debug, "kube/portfwd: %s: %d: %s: connected (took %v)\n", args.Identifier, args.ContainerPort, conn.RemoteAddr(), time.Since(t))
					return handleConnection(ctx, streamConn, conn, 0, args.Identifier, args.ContainerPort)
				})
			}
		})
	}

	return eg.Wait()
}

func dialPod(ctx context.Context, restClient *rest.RESTClient, config *rest.Config, ns, podName string) (httpstream.Connection, error) {
	reqtmpl := restClient.Post().Resource("pods").Namespace(ns).Name(podName).SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqtmpl.URL().String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	streamConn, _, err := spdy.Negotiate(upgrader, &http.Client{Transport: transport}, req, PortForwardProtocolV1Name)
	if err != nil {
		return nil, err
	}

	return streamConn, err
}

func handleConnection(ctx context.Context, streamConn httpstream.Connection, conn net.Conn, requestID int, debugid string, containerPort int) error {
	defer conn.Close()

	localAddr := conn.(*net.TCPConn).LocalAddr().(*net.TCPAddr)
	remoteAddr := conn.(*net.TCPConn).RemoteAddr().(*net.TCPAddr)

	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, fmt.Sprintf("%d", containerPort))
	headers.Set(v1.PortForwardRequestIDHeader, fmt.Sprintf("%d", requestID))

	makeErr := func(msg string, err error) error {
		return fmt.Errorf("kube/portfwd: %s: %d -> %d: %s: %w", debugid, localAddr.Port, containerPort, msg, err)
	}

	errorStream, err := streamConn.CreateStream(headers)
	if err != nil {
		return makeErr("failed to create error stream", err)
	}

	// we're not writing to this stream
	errorStream.Close()

	errorChan := make(chan error)
	go func() {
		defer close(errorChan)

		message, err := ioutil.ReadAll(errorStream)
		switch {
		case err != nil:
			errorChan <- makeErr("error reading from error stream", err)
		case len(message) > 0:
			errorChan <- makeErr("error ocurred during forwarding", fnerrors.New(string(message)))
		}
	}()

	// create data stream
	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := streamConn.CreateStream(headers)
	if err != nil {
		return makeErr("failed to create data stream", err)
	}

	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	go func() {
		// inform the select below that the remote copy is done
		defer close(remoteDone)

		// Copy from the remote side to the local port.
		if _, err := io.Copy(conn, dataStream); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			fmt.Fprintf(console.Warnings(ctx), "%v\n", makeErr("error copying from remote stream to local connection", err))
		}
	}()

	go func() {
		// inform server we're not sending any more data after copy unblocks
		defer dataStream.Close()

		// Copy from the local port to the remote side.
		if _, err := io.Copy(dataStream, conn); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			fmt.Fprintf(console.Warnings(ctx), "%v\n", makeErr("error copying from remote stream to local connection", err))

			// break out of the select below without waiting for the other copy to finish
			close(localError)
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}

	fmt.Fprintf(console.Debug(ctx), "kube/portfwd: %s: %d -> %d: remote addr: %v: closed connection: %v\n", debugid, localAddr.Port, containerPort, remoteAddr, err)

	// always expect something on errorChan (it may be nil)
	return <-errorChan
}

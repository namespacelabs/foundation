// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"go.uber.org/atomic"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport/spdy"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/runtime"
)

type fwdArgs struct {
	Namespace     string
	Identifier    string
	LocalAddrs    []string
	LocalPort     int
	ContainerPort int
	Resolve       func(context.Context) (v1.Pod, error)
	ReportPorts   func(runtime.ForwardedPort)
}

const PortForwardProtocolV1Name = "portforward.k8s.io"

func (r boundEnv) startAndBlockPortFwd(ctx context.Context, args fwdArgs) error {
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

	debug := console.Debug(ctx)

	ids := atomic.NewInt32(0)
	ex, wait := executor.New(ctx)
	for _, localAddr := range args.LocalAddrs {
		var cfg net.ListenConfig
		lst, err := cfg.Listen(ctx, "tcp", fmt.Sprintf("%s:%d", localAddr, args.LocalPort))
		if err != nil {
			return err
		}

		ex.Go(func(ctx context.Context) error {
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

				ex.Go(func(ctx context.Context) error {
					pod, err := args.Resolve(ctx)
					if err != nil {
						return err
					}

					fmt.Fprintf(debug, "kube/portfwd: %s: %d: resolved pod: %s\n", args.Identifier, args.ContainerPort, pod.Name)

					req := restClient.Post().Resource("pods").Namespace(args.Namespace).Name(pod.Name).SubResource("portforward")

					dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

					streamConn, _, err := dialer.Dial(PortForwardProtocolV1Name)
					if err != nil {
						return err
					}

					defer streamConn.Close()

					return handleConnection(ctx, streamConn, conn, int(ids.Inc()), pod.Name, args.ContainerPort, nil)
				})
			}
		})
	}

	return wait()
}

func handleConnection(ctx context.Context, streamConn httpstream.Connection, conn net.Conn, requestID int, podName string, containerPort int, errch chan error) error {
	defer conn.Close()

	localAddr := conn.(*net.TCPConn).LocalAddr().(*net.TCPAddr)
	remoteAddr := conn.(*net.TCPConn).RemoteAddr().(*net.TCPAddr)

	debug := console.Debug(ctx)
	fmt.Fprintf(debug, "kube/portfwd: handling connection for %s:%d: remote addr: %v\n", podName, containerPort, remoteAddr)

	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, fmt.Sprintf("%d", containerPort))
	headers.Set(v1.PortForwardRequestIDHeader, fmt.Sprintf("%d", requestID))

	makeErr := func(msg string, err error) error {
		return fmt.Errorf("kube/portfwd: %s: %d -> %d: %s: %w", podName, localAddr.Port, containerPort, msg, err)
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
			errorChan <- makeErr("error ocurred during forwarding", errors.New(string(message)))
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
			errch <- makeErr("error copying from remote stream to local connection", err)
		}
	}()

	go func() {
		// inform server we're not sending any more data after copy unblocks
		defer dataStream.Close()

		// Copy from the local port to the remote side.
		if _, err := io.Copy(dataStream, conn); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			errch <- makeErr("error copying from local connection to remote stream", err)

			// break out of the select below without waiting for the other copy to finish
			close(localError)
		}
	}()

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	}

	fmt.Fprintf(debug, "kube/portfwd: %s: %d -> %d: remote addr: %v: closed connection: %v\n", podName, localAddr.Port, containerPort, remoteAddr, err)

	// always expect something on errorChan (it may be nil)
	return <-errorChan
}

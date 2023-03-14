// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/sys"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

type unixSockProxy struct {
	TempDir    string
	SocketAddr string
	Cleanup    func()
}

type unixSockProxyOpts struct {
	Kind           string
	SocketPath     string
	Blocking       bool
	Connect        func(context.Context) (net.Conn, error)
	AnnounceSocket func(string)
}

func runUnixSocketProxy(ctx context.Context, clusterId string, opts unixSockProxyOpts) (*unixSockProxy, error) {
	socketPath := opts.SocketPath
	var cleanup func()

	if socketPath == "" {
		sockDir, err := dirs.CreateUserTempDir(opts.Kind, clusterId)
		if err != nil {
			return nil, err
		}

		socketPath = filepath.Join(sockDir, opts.Kind+".sock")
		cleanup = func() {
			os.RemoveAll(socketPath)
		}
	} else {
		cleanup = func() {}
	}

	listener, err := sys.CreateUnixSocket(socketPath)
	if err != nil {
		cleanup()
		return nil, err
	}

	if opts.AnnounceSocket != nil {
		opts.AnnounceSocket(socketPath)
	}

	if opts.Blocking {
		if err := serveProxy(ctx, listener, opts.Connect); err != nil {
			return nil, err
		}

		return nil, nil
	} else {
		go func() {
			if err := serveProxy(ctx, listener, opts.Connect); err != nil {
				log.Fatal(err)
			}
		}()

		return &unixSockProxy{filepath.Dir(socketPath), socketPath, cleanup}, nil
	}
}

func serveProxy(ctx context.Context, listener net.Listener, connect func(context.Context) (net.Conn, error)) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go func() {
			defer conn.Close()

			peerConn, err := connect(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
				return
			}

			defer peerConn.Close()

			go func() {
				_, _ = io.Copy(conn, peerConn)
			}()

			_, _ = io.Copy(peerConn, conn)
		}()
	}
}

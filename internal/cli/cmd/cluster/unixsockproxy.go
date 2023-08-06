// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
	"namespacelabs.dev/foundation/framework/netcopy"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/go-ids"
)

type unixSockProxy struct {
	TempDir    string
	SocketAddr string
	Cleanup    func()
}

type grpcProxyOpts struct {
	SocketPath     string
	Kind           string
	Blocking       bool
	Connect        func(context.Context) (net.Conn, error)
	AnnounceSocket func(string)
}

type unixSockProxyOpts struct {
	SocketPath     string
	Kind           string
	Blocking       bool
	Connect        func(context.Context) (net.Conn, error)
	AnnounceSocket func(string)
}

func runUnixSocketProxy(ctx context.Context, clusterId string, opts unixSockProxyOpts) (*unixSockProxy, error) {
	socketPath := opts.SocketPath
	var cleanup func()

	if socketPath == "" {
		sockDir, err := dirs.CreateUserTempDir("", clusterId)
		if err != nil {
			return nil, err
		}

		socketPath = filepath.Join(sockDir, opts.Kind+".sock")
		cleanup = func() {
			os.RemoveAll(sockDir)
		}
	} else {
		cleanup = func() {}
	}

	if err := unix.Unlink(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var d net.ListenConfig
	listener, err := d.Listen(ctx, "unix", socketPath)
	if err != nil {
		cleanup()
		return nil, err
	}

	if opts.AnnounceSocket != nil {
		opts.AnnounceSocket(socketPath)
	}

	if opts.Blocking {
		defer cleanup()

		ch := make(chan struct{})
		go func() {
			select {
			case <-ch:
			case <-ctx.Done():
			}
			_ = listener.Close()
		}()

		defer close(ch)

		if err := serveProxy(ctx, listener, func(ctx context.Context) (net.Conn, error) {
			return opts.Connect(ctx)
		}); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}

			return nil, err
		}

		return nil, nil
	} else {
		go func() {
			if err := serveProxy(ctx, listener, func(ctx context.Context) (net.Conn, error) {
				return opts.Connect(ctx)
			}); err != nil {
				log.Fatal(err)
			}
		}()

		return &unixSockProxy{filepath.Dir(socketPath), socketPath, cleanup}, nil
	}
}

const debug = false

func serveProxy(ctx context.Context, listener net.Listener, connect func(context.Context) (net.Conn, error)) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go func() {
			var d netcopy.DebugLogFunc
			if debug {
				id := ids.NewRandomBase32ID(8)
				d = func(format string, args ...any) {
					fmt.Fprintf(console.Stderr(ctx), "["+id+"]: "+format+"\n", args...)
				}
			}

			defer conn.Close()

			peerConn, err := connect(ctx)
			if err != nil {
				fmt.Fprintf(console.Stderr(ctx), "Failed to connect: %v\n", err)
				return
			}

			defer peerConn.Close()

			_ = netcopy.CopyConns(d, conn, peerConn)
		}()
	}
}

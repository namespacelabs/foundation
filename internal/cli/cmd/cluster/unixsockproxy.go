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

	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

type unixSockProxy struct {
	TempDir    string
	SocketAddr string
	Cleanup    func()
}

func runUnixSocketProxy(ctx context.Context, kind, clusterId string, connect func(context.Context) (net.Conn, error)) (*unixSockProxy, error) {
	sockDir, err := dirs.CreateUserTempDir(kind, clusterId)
	if err != nil {
		return nil, err
	}

	sockFile := filepath.Join(sockDir, kind+".sock")
	listener, err := net.Listen("unix", sockFile)
	if err != nil {
		os.RemoveAll(sockDir)
		return nil, err
	}

	go serveProxy(ctx, listener, connect)

	return &unixSockProxy{sockDir, sockFile, func() { _ = os.RemoveAll(sockDir) }}, nil
}

func serveProxy(ctx context.Context, listener net.Listener, connect func(context.Context) (net.Conn, error)) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
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

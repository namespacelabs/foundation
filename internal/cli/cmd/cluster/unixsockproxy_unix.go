// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build unix
// +build unix

package cluster

import (
	"context"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func crossPlatformUnixSocketProxyListen(ctx context.Context, socketPath *string, clusterId string, kind string) (net.Listener, func(), error) {
	var cleanup func()

	if *socketPath == "" {
		sockDir, err := dirs.CreateUserTempDir("", clusterId)
		if err != nil {
			return nil, nil, err
		}

		*socketPath = filepath.Join(sockDir, kind+".sock")
		cleanup = func() {
			os.RemoveAll(sockDir)
		}
	} else {
		cleanup = func() {}
	}

	if err := unix.Unlink(*socketPath); err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}

	var d net.ListenConfig
	listener, err := d.Listen(ctx, "unix", *socketPath)
	if err != nil {
		cleanup()
		return nil, nil, err
	}

	return listener, cleanup, err
}

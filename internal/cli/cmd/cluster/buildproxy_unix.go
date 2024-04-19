// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build unix
// +build unix

package cluster

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func toDockerUrl(path string) string {
	return "unix://" + path
}

func crossPlatformListen(path string) (net.Listener, error) {
	if err := unix.Unlink(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	return l, nil
}

func internalListenProxy(ctx context.Context, socketPath *string, platform string) (net.Listener, func() error, error) {
	var cleanup func() error
	if *socketPath == "" {
		sockDir, err := dirs.CreateUserTempDir("", fmt.Sprintf("buildkit.%v", platform))
		if err != nil {
			return nil, nil, err
		}

		*socketPath = filepath.Join(sockDir, "buildkit.sock")
		cleanup = func() error {
			return os.RemoveAll(sockDir)
		}
	} else {
		if err := unix.Unlink(*socketPath); err != nil && !os.IsNotExist(err) {
			return nil, nil, err
		}
	}

	var d net.ListenConfig
	listener, err := d.Listen(ctx, "unix", *socketPath)
	if err != nil {
		if cleanup != nil {
			_ = cleanup()
		}

		return nil, nil, err
	}

	return listener, cleanup, nil
}

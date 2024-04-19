// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build windows
// +build windows

package cluster

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/google/uuid"
)

func toDockerUrl(path string) string {
	return "npipe://" + strings.ReplaceAll(path, "\\", "/")
}

func crossPlatformListen(path string) (net.Listener, error) {
	// From Microsoft docs: An instance of a named pipe is always deleted when the last handle to the instance of the named pipe is closed.
	// https://learn.microsoft.com/en-gb/windows/win32/api/winbase/nf-winbase-createnamedpipea
	return winio.ListenPipe(path, &winio.PipeConfig{})
}

func internalListenProxy(ctx context.Context, socketPath *string, platform string) (net.Listener, func() error, error) {
	if *socketPath == "" {
		*socketPath = filepath.Join("\\\\.\\pipe\\", fmt.Sprintf("nsc-%v-buildkit.%v", uuid.New().String(), platform))
	}

	// As named pipes can't be removed on windows, no need to check for existence prior to listening.
	// If the named pipe already exists, and another process listens it will fail then.
	listener, err := crossPlatformListen(*socketPath)

	if err != nil {
		return nil, nil, err
	}

	return listener, func() error { return nil }, nil
}

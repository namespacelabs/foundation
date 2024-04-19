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

	"github.com/google/uuid"
)

func crossPlatformUnixSocketProxyListen(ctx context.Context, socketPath *string, clusterId string, kind string) (net.Listener, func(), error) {
	if *socketPath == "" {
		*socketPath = filepath.Join("\\\\.\\pipe\\", fmt.Sprintf("nsc-%v-%v-%v", uuid.New().String(), clusterId, kind))
	}

	// As named pipes can't be removed on windows, no need to check for existence prior to listening.
	// If the named pipe already exists, and another process listens it will fail then.
	listener, err := crossPlatformListen(*socketPath)

	if err != nil {
		return nil, nil, err
	}

	return listener, func() {}, nil
}

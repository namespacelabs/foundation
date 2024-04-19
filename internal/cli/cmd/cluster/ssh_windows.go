// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build windows
// +build windows

package cluster

import (
	"context"

	c "github.com/containerd/console"
	"golang.org/x/crypto/ssh"
)

func listenForResize(ctx context.Context, stdin c.Console, session *ssh.Session) {
	// listen for resize is not available on windows.
}

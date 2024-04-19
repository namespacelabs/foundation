// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build unix
// +build unix

package cluster

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	c "github.com/containerd/console"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func listenForResize(ctx context.Context, stdin c.Console, session *ssh.Session) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)

	defer func() {
		signal.Stop(sig)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sig:
		}

		w, h, err := term.GetSize(int(stdin.Fd()))
		if err == nil {
			session.WindowChange(h, w)
		}
	}
}

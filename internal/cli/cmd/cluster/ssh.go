// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/console"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	con "namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [cluster-id]",
		Short: "Start an SSH session.",
		Args:  cobra.MaximumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, err := selectCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusterList) {
				printCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		return inlineSsh(ctx, cluster, nil)
	})

	return cmd
}

func inlineSsh(ctx context.Context, cluster *api.KubernetesCluster, args []string) error {
	stdin, err := console.ConsoleFromFile(os.Stdin)
	if err != nil {
		return err
	}

	if !isatty.IsTerminal(stdin.Fd()) {
		return fnerrors.New("stdin is not a tty")
	}

	signer, err := ssh.ParsePrivateKey(cluster.SshPrivateKey)
	if err != nil {
		return err
	}

	peerConn, err := api.DialPort(ctx, cluster, 22)
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
	}

	c, chans, reqs, err := ssh.NewClientConn(peerConn, "internal", config)
	if err != nil {
		return err
	}

	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}

	session.Stdin = stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	defer session.Close()

	done := con.EnterInputMode(ctx)
	defer done()

	if err := stdin.SetRaw(); err != nil {
		return err
	}

	defer stdin.Reset()

	w, h, err := term.GetSize(int(stdin.Fd()))
	if err != nil {
		return err
	}

	go listenForResize(ctx, stdin, session)

	if err := session.RequestPty("xterm", h, w, nil); err != nil {
		return err
	}

	if err := session.Shell(); err != nil {
		return err
	}

	return session.Wait()
}

func listenForResize(ctx context.Context, stdin console.Console, session *ssh.Session) {
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

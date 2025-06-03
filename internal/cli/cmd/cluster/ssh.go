// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	c "github.com/containerd/console"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	con "namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [instance-id] [command]",
		Short: "Start an SSH session.",
		Args:  cobra.ArbitraryArgs,
	}

	tag := cmd.Flags().String("unique_tag", "", "If specified, creates a instance with the specified unique tag.")
	oneshot := cmd.Flags().Bool("oneshot", false, "If specified, a temporary instance will be created and destroyed upon disconnection.")
	sshAgent := cmd.Flags().BoolP("ssh_agent", "A", false, "If specified, forwards the local SSH agent.")
	forcePty := cmd.Flags().BoolP("force-pty", "t", false, "Force pseudo-terminal allocation.")
	disablePty := cmd.Flags().BoolP("disable-pty", "T", false, "Disable pseudo-terminal allocation.")

	waitTimeout := cmd.Flags().Duration("wait_timeout", time.Minute, "For how long to wait until the instance becomes ready.")
	cmd.Flags().MarkHidden("wait_timeout")

	user := cmd.Flags().String("user", "", "The user to connect as.")
	cmd.Flags().MarkHidden("user")

	computeAPI := cmd.Flags().Bool("compute_api", true, "Whether to use the Compute API.")
	cmd.Flags().MarkHidden("compute_api")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *forcePty && *disablePty {
			return errors.New("Can not use -t and -T")
		}
		sshOpts := InlineSshOpts{
			User:            *user,
			ForwardSshAgent: *sshAgent,
			ForcePty:        *forcePty,
			DisablePty:      *disablePty,
		}
		if *tag != "" {
			opts := api.CreateClusterOpts{
				KeepAtExit:      true,
				Purpose:         fmt.Sprintf("Manually created for ssh (%s)", *tag),
				UniqueTag:       *tag,
				WaitClusterOpts: api.WaitClusterOpts{WaitForService: "ssh", WaitKind: "kubernetes"},
			}

			cluster, err := api.CreateAndWaitCluster(ctx, api.Methods, time.Minute, opts)
			if err != nil {
				return err
			}

			return InlineSsh(ctx, cluster.Cluster, sshOpts, args)
		}
		if *oneshot {
			opts := api.CreateClusterOpts{
				KeepAtExit: false,
				Purpose:    fmt.Sprintf("Temporary instance for SSH"),
				WaitClusterOpts: api.WaitClusterOpts{
					WaitForService: "ssh",
					WaitKind:       "kubernetes",
				},
				Duration:      time.Minute,
				UseComputeAPI: *computeAPI,
			}

			cluster, err := api.CreateAndWaitCluster(ctx, api.Methods, *waitTimeout, opts)
			if err != nil {
				return err
			}

			return InlineSsh(ctx, cluster.Cluster, sshOpts, args)
		}

		cluster, args, err := SelectRunningCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusterList) {
				PrintCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		return InlineSsh(ctx, cluster, sshOpts, args)
	})
	cmd.Flags().MarkHidden("tmp")

	return cmd
}

func NewTopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top [instance-id]",
		Short: "Observe resource utilization of the target instance.",
		Args:  cobra.MaximumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := SelectRunningCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusterList) {
				PrintCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		return InlineSsh(ctx, cluster, InlineSshOpts{}, []string{"/bin/sh", "-c", "command -v htop > /dev/null && htop || top"})
	})

	return cmd
}

func withSsh(ctx context.Context, cluster *api.KubernetesCluster, user string, callback func(context.Context, *ssh.Client) error) error {
	sshSvc := api.ClusterService(cluster, "ssh")
	if sshSvc == nil || sshSvc.Endpoint == "" {
		return fnerrors.Newf("instance does not have ssh")
	}

	if sshSvc.Status != "READY" {
		return fnerrors.Newf("expected ssh to be READY, saw %q", sshSvc.Status)
	}

	signer, err := ssh.ParsePrivateKey(cluster.SshPrivateKey)
	if err != nil {
		return err
	}

	peerConn, err := api.DialEndpoint(ctx, sshSvc.Endpoint)
	if err != nil {
		return err
	}

	if user == "" {
		user = "root"
	}

	config := &ssh.ClientConfig{
		User: user,
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

	return callback(ctx, client)
}

type InlineSshOpts struct {
	User            string
	ForwardSshAgent bool
	ForcePty        bool
	DisablePty      bool
}

func InlineSsh(ctx context.Context, cluster *api.KubernetesCluster, opts InlineSshOpts, args []string) error {
	wantPty := !opts.DisablePty && ((len(args) == 0) || opts.ForcePty)

	hasLocalPty := false
	var localPty c.Console

	if wantPty && !isatty.IsTerminal(os.Stdin.Fd()) {
		fmt.Fprintln(console.Debug(ctx), "Pseudo-terminal will not be allocated because stdin is not a terminal.")
	}

	if wantPty {
		var err error
		localPty, err = c.ConsoleFromFile(os.Stdin)
		if err != nil {
			fmt.Printf("Could not get console from stdin: %v", err)
		} else {
			hasLocalPty = true
		}
	}

	return withSsh(ctx, cluster, opts.User, func(ctx context.Context, client *ssh.Client) error {
		session, err := client.NewSession()
		if err != nil {
			return err
		}

		if opts.ForwardSshAgent {
			if authSock := os.Getenv("SSH_AUTH_SOCK"); authSock != "" {
				if err := agent.ForwardToRemote(client, authSock); err != nil {
					return err
				}

				if err := agent.RequestAgentForwarding(session); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(console.Warnings(ctx), "ssh-agent forwarding requested, without a SSH_AUTH_SOCK\n")
			}
		}

		session.Stdin = os.Stdin
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr

		defer session.Close()

		if hasLocalPty {
			done := con.EnterInputMode(ctx)
			defer done()

			if err := localPty.SetRaw(); err != nil {
				return err
			}

			defer localPty.Reset()

			w, h, err := term.GetSize(int(localPty.Fd()))
			if err != nil {
				return err
			}

			go listenForResize(ctx, localPty, session)

			if err := session.RequestPty("xterm", h, w, nil); err != nil {
				return err
			}
		}

		if len(args) > 0 {
			command := strings.Join(args, " ")
			if err := session.Run(command); err != nil {
				return err
			}
			return nil
		} else {
			if err := session.Shell(); err != nil {
				return err
			}

			g := executor.New(ctx, "ssh")
			cancel := g.GoCancelable(func(ctx context.Context) error {
				return api.StartRefreshing(ctx, api.Methods, cluster, func(err error) error {
					fmt.Fprintf(os.Stderr, "failed to refresh instance: %v\n", err)
					return nil
				})
			})
			g.Go(func(ctx context.Context) error {
				defer cancel()
				return session.Wait()
			})
			return g.Wait()
		}
	})
}

func listenForResize(ctx context.Context, stdin c.Console, session *ssh.Session) {
	sig := make(chan os.Signal, 1)
	termios.NotifyWindowSize(sig)

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

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
	"strings"

	c "github.com/containerd/console"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	con "namespacelabs.dev/foundation/internal/console"
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
	sshAgent := cmd.Flags().BoolP("ssh_agent", "A", false, "If specified, forwards the local SSH agent.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *tag != "" {
			opts := api.CreateClusterOpts{
				KeepAtExit:      true,
				Purpose:         fmt.Sprintf("Manually created for ssh (%s)", *tag),
				UniqueTag:       *tag,
				WaitClusterOpts: api.WaitClusterOpts{WaitForService: "ssh", WaitKind: "kubernetes"},
			}

			cluster, err := api.CreateAndWaitCluster(ctx, api.Methods, opts)
			if err != nil {
				return err
			}

			return InlineSsh(ctx, cluster.Cluster, *sshAgent, args)
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

		return InlineSsh(ctx, cluster, *sshAgent, args)
	})

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

		return InlineSsh(ctx, cluster, false, []string{"/bin/sh", "-c", "command -v htop > /dev/null && htop || top"})
	})

	return cmd
}

func withSsh(ctx context.Context, cluster *api.KubernetesCluster, callback func(context.Context, *ssh.Client) error) error {
	sshSvc := api.ClusterService(cluster, "ssh")
	if sshSvc == nil || sshSvc.Endpoint == "" {
		return fnerrors.New("instance does not have ssh")
	}

	if sshSvc.Status != "READY" {
		return fnerrors.New("expected ssh to be READY, saw %q", sshSvc.Status)
	}

	signer, err := ssh.ParsePrivateKey(cluster.SshPrivateKey)
	if err != nil {
		return err
	}

	peerConn, err := api.DialEndpoint(ctx, sshSvc.Endpoint)
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

	return callback(ctx, client)
}

func InlineSsh(ctx context.Context, cluster *api.KubernetesCluster, sshAgent bool, args []string) error {
	stdin, err := c.ConsoleFromFile(os.Stdin)
	if err != nil {
		return err
	}

	if !isatty.IsTerminal(stdin.Fd()) {
		return fnerrors.New("stdin is not a tty")
	}

	return withSsh(ctx, cluster, func(ctx context.Context, client *ssh.Client) error {
		session, err := client.NewSession()
		if err != nil {
			return err
		}

		if sshAgent {
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
				return api.StartRefreshing(ctx, api.Methods, cluster.ClusterId, func(err error) error {
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

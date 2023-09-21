// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package private

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	c "github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newInternalTerminalAttach() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Attach to an existing terminal.",
		Args:  cobra.ArbitraryArgs,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return fnerrors.New("need at least one command to run")
		}

		cli, err := containerd.New("/var/run/containerd/containerd.sock")
		if err != nil {
			return err
		}

		containers, err := cli.Containers(namespaces.WithNamespace(ctx, "default"), fmt.Sprintf("labels.%q==%s", "nsc/ctr-container-type", "terminal-source"))
		if err != nil {
			return err
		}

		if len(containers) == 0 {
			return errors.New("no terminal source registered")
		}

		target := containers[0]

		execArgs := []string{
			"exec", "-it",
		}

		if authSock := os.Getenv("SSH_AUTH_SOCK"); authSock != "" {
			execArgs = append(execArgs, "-e", "SSH_AUTH_SOCK="+authSock)
		}

		for _, envvar := range []string{"NSC_ENDPOINT", "NSC_TOKEN_SPEC_FILE"} {
			if v := os.Getenv(envvar); v != "" {
				execArgs = append(execArgs, "-e", fmt.Sprintf("%s=%s", envvar, v))
			}
		}

		execArgs = append(execArgs, target.ID())
		execArgs = append(execArgs, args...)

		stdin, err := c.ConsoleFromFile(os.Stdin)
		if err != nil {
			return err
		}

		cmd := exec.CommandContext(ctx, "nerdctl", execArgs...)

		cmd.Stdin = stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		done := console.EnterInputMode(ctx)
		defer done()

		if err := stdin.SetRaw(); err != nil {
			return err
		}

		defer stdin.Reset()

		return cmd.Run()
	})

	return cmd
}

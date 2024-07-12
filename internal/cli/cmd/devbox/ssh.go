// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newSshCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh tag",
		Short: "ssh into a devbox.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return sshDevbox(ctx, args[0])
	})

	return cmd
}

func sshDevbox(ctx context.Context, tag string) error {
	devboxClient, err := getDevBoxClient(ctx)
	if err != nil {
		return err
	}

	instance, err := doEnsureDevbox(ctx, devboxClient, tag)
	if err != nil {
		return err
	}

	if err := offerSetupSshAgentForwarding(ctx); err != nil {
		return err
	}

	cmd := exec.Command("ssh", instance.regionalSshEndpoint)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

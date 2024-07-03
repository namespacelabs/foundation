// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
)

func newVscodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code tag",
		Short: "Initiate a vscode session using a devbox.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return vscodeDevbox(ctx, args[0])
	})

	return cmd
}

func vscodeDevbox(ctx context.Context, tag string) error {
	_, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("Could not find 'code' - please install vscode first.")
	}

	// TODO: Maybe use --list-extension
	// and --install-extension <ext>
	// To ensure that the SSH remote extension is installed.

	devboxClient, err := getDevBoxClient(ctx)
	if err != nil {
		return err
	}

	devbox, err := getSingleDevbox(ctx, devboxClient, tag)
	if err != nil {
		return err
	}

	// Probably run something like code --remote ssh-remote+<host> <folder>
	// The folder could be passed as an argument and default to "~"
	fmt.Fprintf(console.Stdout(ctx), "TODO: actually code %s!\n", devbox.GetDevboxStatus().GetSshEndpoint())

	return nil
}

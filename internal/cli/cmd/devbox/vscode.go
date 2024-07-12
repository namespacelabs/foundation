// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

const (
	// TODO this should not be hardcoded long term
	DEVBOX_HOME_DIR = "/home/dev"
)

func newVscodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code tag [path]",
		Short: "Initiate a vscode session on the devbox 'tag'.",
		Long:  "Initiate a vscode session on the devbox 'tag'. If 'path' is given, vscode remote is opened in that path on the devbox. Otherwise the home directory of the devbox is used as the path.",
		Args:  cobra.RangeArgs(1, 2),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		// TODO evaluate if supporting "open specific folder on remote" is useful
		pathOnRemote := "."
		if len(args) >= 2 {
			pathOnRemote = args[1]
		}
		return vscodeDevbox(ctx, args[0], pathOnRemote)
	})

	return cmd
}

func vscodeDevbox(ctx context.Context, tag string, pathOnRemote string) error {
	_, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("Could not find 'code' - please install vscode first.")
	}

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

	// https://code.visualstudio.com/docs/remote/troubleshooting#_connect-to-a-remote-host-from-the-terminal
	// Note that vscode will offer to install the necessary extension if it's not installed yet.
	vscodeRemoteSpec := "ssh-remote+" + instance.regionalSshEndpoint
	absPathOnRemote := filepath.Join(DEVBOX_HOME_DIR, pathOnRemote)

	cmd := exec.Command("code", "--remote", vscodeRemoteSpec, absPathOnRemote)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

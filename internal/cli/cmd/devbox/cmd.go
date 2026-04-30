// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	docsURL           = "https://namespace.so/docs/devbox"
	unixInstallCmd    = "curl -fsSL get.namespace.so/devbox/install.sh | bash"
	windowsInstallCmd = "irm https://get.namespace.so/devbox/install.ps1 | iex"
)

func NewDevboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "devbox",
		Short:  "Devboxes are managed by the separate devbox CLI.",
		Hidden: true,
		Args:   cobra.NoArgs,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		out := console.Stdout(ctx)

		fmt.Fprintln(out, "Devboxes are not managed through nsc — use the separate `devbox` CLI.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Install:")
		if runtime.GOOS == "windows" {
			fmt.Fprintf(out, "  %s\n", windowsInstallCmd)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "On Linux/macOS:")
			fmt.Fprintf(out, "  %s\n", unixInstallCmd)
		} else {
			fmt.Fprintf(out, "  %s\n", unixInstallCmd)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "On Windows (PowerShell):")
			fmt.Fprintf(out, "  %s\n", windowsInstallCmd)
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, "For full documentation, including how to create and manage devboxes, see %s\n", docsURL)

		return fnerrors.ExitWithCode(errors.New(""), 1)
	})

	return cmd
}

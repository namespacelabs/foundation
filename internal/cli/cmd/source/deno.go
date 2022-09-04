// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/sdk/deno"
	"namespacelabs.dev/foundation/runtime/rtypes"
)

func newDenoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deno",
		Short: "Run Namespace-configured deno.",
	}

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env planning.Context, args []string) error {
		d, err := deno.EnsureSDK(ctx)
		if err != nil {
			return err
		}

		if err := d.CacheImports(ctx, env.WorkspaceLoadedFrom().AbsPath); err != nil {
			return err
		}

		x := console.EnterInputMode(ctx)
		defer x()

		return d.Run(ctx, env.WorkspaceLoadedFrom().AbsPath, rtypes.IO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}, "repl", "--cached-only")
	})
}

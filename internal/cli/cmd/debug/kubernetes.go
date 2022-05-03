// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/workspace/module"
)

func newKubernetesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "kubernetes",
	}

	envBound := "dev"
	systemInfo := &cobra.Command{
		Use:  "system-info",
		Args: cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envBound)
			if err != nil {
				return err
			}

			k, err := kubernetes.New(ctx, root.Workspace, root.DevHost, env.Proto())
			if err != nil {
				return err
			}

			sysInfo, err := k.SystemInfo(ctx)
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), prototext.Format(sysInfo))
			return nil
		}),
	}

	systemInfo.Flags().StringVar(&envBound, "env", envBound, "If specified, produce a env-bound sealed schema.")

	cmd.AddCommand(systemInfo)

	return cmd
}

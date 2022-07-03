// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func newGenerateTokenCmd() *cobra.Command {
	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "generate-token",
		Short: "Generates a EKS session token.",
		Args:  cobra.ExactArgs(1),
	}, func(ctx context.Context, env provision.Env, args []string) error {
		s, err := eks.NewSession(ctx, env.DevHost(), devhost.ByEnvironment(env.Proto()))
		if err != nil {
			return err
		}

		token, err := eks.ComputeBearerToken(ctx, s, args[0])
		if err != nil {
			return err
		}

		fmt.Fprintln(console.Stdout(ctx), token)
		return nil
	})

	return cmd
}

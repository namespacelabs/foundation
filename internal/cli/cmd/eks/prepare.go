// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
)

func newPrepareCmd() *cobra.Command {
	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "prepare-ingress",
		Short: "Runs ingress preparation.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env provision.Env, args []string) error {
		rt, err := kubernetes.NewFromEnv(ctx, env)
		if err != nil {
			return err
		}

		return prepare.PrepareIngressInKube(ctx, env, rt)
	})

	return cmd
}

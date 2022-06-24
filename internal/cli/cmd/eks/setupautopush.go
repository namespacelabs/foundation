// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func NewSetupAutopushCmd() *cobra.Command {
	var iamRole string
	var dryRun bool

	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "setup-autopush",
		Short: "Sets up production cluster for automatic deployments to a staging environment.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env provision.Env, args []string) error {
		s, err := eks.NewSession(ctx, env.DevHost(), devhost.ByEnvironment(env.Proto()))
		if err != nil {
			return err
		}

		eksCluster, err := eks.PrepareClusterInfo(ctx, s)
		if err != nil {
			return err
		}

		if eksCluster == nil {
			return fnerrors.New("not an eks cluster")
		}

		result, err := eks.SetupAutopush(eksCluster, iamRole)
		if err != nil {
			return err
		}

		p := ops.NewPlan()
		for _, inv := range result {
			def, err := inv.ToDefinition()
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Fprintln(console.Stdout(ctx), prototext.Format(def))
			} else {
				if err := p.Add(def); err != nil {
					return err
				}
			}
		}
		if dryRun {
			fmt.Fprintf(console.Stdout(ctx), "Not making changes to the cluster, as --dry_run=true.\n\n")
		} else {
			if _, err := p.Execute(ctx, "eks.autopush.apply", env); err != nil {
				return err
			}
		}
		return nil
	})

	cmd.Flags().StringVar(&iamRole, "iam_role", "namespace-ci", "IAM Role to create.")
	cmd.Flags().BoolVar(&dryRun, "dry_run", true, "If true, print invocations, rather than executing them.")

	return cmd
}

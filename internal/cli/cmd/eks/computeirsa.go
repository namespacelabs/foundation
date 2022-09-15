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
	"namespacelabs.dev/foundation/std/planning"
)

func newComputeIrsaCmd() *cobra.Command {
	var iamRole, namespace, serviceAccount string
	var dryRun bool

	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "compute-irsa",
		Short: "Sets up IRSA for the specified IAM role and Service Account.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env planning.Context, args []string) error {
		s, err := eks.NewSession(ctx, env.Configuration())
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

		result, err := eks.PrepareIrsa(eksCluster, iamRole, namespace, serviceAccount, nil)
		if err != nil {
			return err
		}

		p := ops.NewEmptyPlan()
		for _, inv := range result.Invocations {
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
			if err := ops.Execute(ctx, env.Configuration(), "eks.irsa.apply", p, nil); err != nil {
				return err
			}
		}

		for _, ext := range result.Extensions {
			def, err := ext.ToDefinition()
			if err != nil {
				return err
			}

			fmt.Fprintln(console.Stdout(ctx), prototext.Format(def))
		}

		return err
	})

	cmd.Flags().StringVar(&iamRole, "iam_role", "", "IAM Role to manage.")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace where the service account lives.")
	cmd.Flags().StringVar(&serviceAccount, "service_account", "", "Which service account to bind to IAM role.")
	cmd.Flags().BoolVar(&dryRun, "dry_run", true, "If true, print invocations, rather than executing them.")

	_ = cmd.MarkFlagRequired("iam_role")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("service_account")

	return cmd
}

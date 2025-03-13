// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package eks

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/aws/eks"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	fneks "namespacelabs.dev/foundation/universe/aws/eks"
)

func newComputeIrsaCmd() *cobra.Command {
	var iamRole, namespace, serviceAccount string
	var dryRun bool

	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "compute-irsa",
		Short: "Sets up IRSA for the specified IAM role and Service Account.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env cfg.Context, args []string) error {
		s, err := eks.NewSession(ctx, env.Configuration())
		if err != nil {
			return err
		}

		eksCluster, err := eks.PrepareClusterInfo(ctx, s)
		if err != nil {
			return err
		}

		if eksCluster == nil {
			return fnerrors.Newf("not an eks cluster")
		}

		result, err := fneks.PrepareIrsa(eksCluster, iamRole, namespace, serviceAccount, nil)
		if err != nil {
			return err
		}

		p := execution.NewEmptyPlan()
		for _, inv := range result.Invocations {
			def, err := inv.ToDefinition()
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Fprintln(console.Stdout(ctx), prototext.Format(def))
			} else {
				p.Add(def)
			}
		}

		if dryRun {
			fmt.Fprintf(console.Stdout(ctx), "Not making changes to the cluster, as --dry_run=true.\n\n")
		} else {
			if err := execution.Execute(ctx, "eks.irsa.apply", p, nil, execution.FromContext(env)); err != nil {
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

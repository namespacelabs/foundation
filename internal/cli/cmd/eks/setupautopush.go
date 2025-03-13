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
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/internal/providers/aws"
	"namespacelabs.dev/foundation/internal/providers/aws/auth"
	"namespacelabs.dev/foundation/internal/providers/aws/eks"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
)

const appURL = "https://github.com/apps/obsolete-namespace-ci/installations/new"

func newSetupAutopushCmd() *cobra.Command {
	var iamRole string
	var dryRun bool

	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "setup-autopush",
		Short: "Sets up production cluster for automatic deployments to a staging environment.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env cfg.Context, args []string) error {
		cluster, err := runtime.ClusterFor(ctx, env)
		if err != nil {
			return err
		}

		acc, err := getAwsAccount(ctx, env)
		if err != nil {
			return err
		}
		roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", acc, iamRole)

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

		result, err := eks.SetupAutopush(eksCluster, iamRole, roleArn)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		p := execution.NewEmptyPlan()
		for _, inv := range result {
			def, err := inv.ToDefinition()
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Fprintln(stdout, prototext.Format(def))
			} else {
				p.Add(def)
			}
		}
		if dryRun {
			fmt.Fprintf(stdout, "Not making changes to the cluster, as --dry_run=true.\n\n")
		} else {
			if err := execution.Execute(ctx, "eks.autopush.apply", p, nil, execution.FromContext(env), runtime.ClusterInjection.With(cluster)); err != nil {
				return err
			}
		}

		fmt.Fprintf(stdout, "Success!\nNext steps:\n")
		fmt.Fprintf(stdout, " 1. Please inform a Namespace Labs dev that %q has been set up for autopush so they can whitelist it for deployment. %s\n", roleArn, colors.Ctx(ctx).Comment.Apply("This step will be automated in future."))
		fmt.Fprintf(stdout, " 2. Install %s into your Github repository.\n", appURL)

		return nil
	})

	cmd.Flags().StringVar(&iamRole, "iam_role", "namespace-ci", "IAM Role to create.")
	cmd.Flags().BoolVar(&dryRun, "dry_run", true, "If true, print invocations, rather than executing them.")

	return cmd
}

func getAwsAccount(ctx context.Context, env cfg.Context) (string, error) {
	cfg, err := awsprovider.MustConfiguredSession(ctx, env.Configuration())
	if err != nil {
		return "", err
	}
	caller := auth.ResolveWithConfig(cfg)
	res, err := compute.Get(ctx, caller)
	if err != nil {
		return "", err
	}

	if res.Value.Account == nil {
		return "", fmt.Errorf("unable to fetch AWS account")
	}

	return *res.Value.Account, nil
}

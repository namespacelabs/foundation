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
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/providers/aws/auth"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
)

const appURL = "https://github.com/apps/namespace-continuous-integration/installations/new"

func newSetupAutopushCmd() *cobra.Command {
	var iamRole string
	var dryRun bool

	cmd := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "setup-autopush",
		Short: "Sets up production cluster for automatic deployments to a staging environment.",
		Args:  cobra.NoArgs,
	}, func(ctx context.Context, env planning.Context, args []string) error {
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
			return fnerrors.New("not an eks cluster")
		}

		result, err := eks.SetupAutopush(eksCluster, iamRole, roleArn)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		p := ops.NewEmptyPlan()
		for _, inv := range result {
			def, err := inv.ToDefinition()
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Fprintln(stdout, prototext.Format(def))
			} else {
				if err := p.Add(def); err != nil {
					return err
				}
			}
		}
		if dryRun {
			fmt.Fprintf(stdout, "Not making changes to the cluster, as --dry_run=true.\n\n")
		} else {
			if _, err := ops.Execute(ctx, env.Configuration(), "eks.autopush.apply", p, runtime.ClusterInjection.With(cluster)); err != nil {
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

func getAwsAccount(ctx context.Context, env planning.Context) (string, error) {
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

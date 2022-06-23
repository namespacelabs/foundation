// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"encoding/json"
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

func NewEksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "eks",
		Short:  "EKS-related activities (internal only).",
		Hidden: true,
	}

	var iamRole, namespace, serviceAccount string
	var dryRun bool

	computeIrsa := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "compute-irsa",
		Short: "Sets up IRSA for the specified IAM role and Service Account.",
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

		result, err := eks.PrepareIrsa(eksCluster, iamRole, namespace, serviceAccount, nil)
		if err != nil {
			return err
		}

		p := ops.NewPlan()
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
			if _, err := p.Execute(ctx, "eks.irsa.apply", env); err != nil {
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

	computeIrsa.Flags().StringVar(&iamRole, "iam_role", "", "IAM Role to manage.")
	computeIrsa.Flags().StringVar(&namespace, "namespace", "", "Namespace where the service account lives.")
	computeIrsa.Flags().StringVar(&serviceAccount, "service_account", "", "Which service account to bind to IAM role.")
	computeIrsa.Flags().BoolVar(&dryRun, "dry_run", true, "If true, print invocations, rather than executing them.")

	_ = computeIrsa.MarkFlagRequired("iam_role")
	_ = computeIrsa.MarkFlagRequired("namespace")
	_ = computeIrsa.MarkFlagRequired("service_account")

	generateToken := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "generate-token",
		Short: "Generates a EKS session token.",
		Args:  cobra.ExactArgs(1),
	}, func(ctx context.Context, env provision.Env, args []string) error {
		s, err := eks.NewSession(ctx, env.DevHost(), devhost.ByEnvironment(env.Proto()))
		if err != nil {
			return err
		}

		token, err := eks.ComputeToken(ctx, s, args[0])
		if err != nil {
			return err
		}

		fmt.Fprintln(console.Stdout(ctx), token)
		return nil
	})

	generateConfig := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "kube-config",
		Short: "Generates a EKS kubeconfig.",
		Args:  cobra.ExactArgs(1),
	}, func(ctx context.Context, env provision.Env, args []string) error {
		s, err := eks.NewSession(ctx, env.DevHost(), devhost.ByEnvironment(env.Proto()))
		if err != nil {
			return err
		}

		cluster, err := eks.DescribeCluster(ctx, s, args[0])
		if err != nil {
			return err
		}

		cfg, err := eks.Kubeconfig(cluster, "")
		if err != nil {
			return err
		}

		w := json.NewEncoder(console.Stdout(ctx))
		w.SetIndent("", "  ")
		w.Encode(cfg)
		return nil
	})

	cmd.AddCommand(computeIrsa)
	cmd.AddCommand(generateToken)
	cmd.AddCommand(generateConfig)

	return cmd
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewDeployPlanCmd() *cobra.Command {
	var opts deployOpts

	cmd := &cobra.Command{
		Use:    "deploy-plan <path/to/plan>",
		Short:  "Deploys a previously serialized plan.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.Flags().BoolVar(&opts.alsoWait, "wait", true, "Wait for the deployment after running.")
	cmd.Flags().StringVar(&opts.outputPath, "output_to", "", "If set, a machine-readable output is emitted after successful deployment.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		raw, err := os.ReadFile(args[0])
		if err != nil {
			return fnerrors.New("failed to load %q: %w", args[0], err)
		}

		plan := &schema.DeployPlan{}
		if err := proto.Unmarshal(raw, plan); err != nil {
			return fnerrors.New("failed to unmarshal %q: %w", args[0], err)
		}

		p, err := ops.NewPlan(plan.GetProgram().GetInvocation()...)
		if err != nil {
			return fnerrors.New("failed to prepare plan: %w", err)
		}

		config, err := planning.MakeConfigurationCompat(root, root.Workspace(), root.DevHost(), plan.Environment)
		if err != nil {
			return err
		}

		env := serializedContext{root, config, plan.Environment}

		cluster, err := runtime.ClusterFor(ctx, env)
		if err != nil {
			return err
		}

		return completeDeployment(ctx, env, cluster, p, plan, opts)
	})

	return cmd
}

type serializedContext struct {
	root   *workspace.Root
	config planning.Configuration
	env    *schema.Environment
}

func (se serializedContext) Workspace() planning.Workspace         { return se.root.Workspace() }
func (se serializedContext) Environment() *schema.Environment      { return se.env }
func (se serializedContext) ErrorLocation() string                 { return se.root.ErrorLocation() }
func (se serializedContext) Configuration() planning.Configuration { return se.config }

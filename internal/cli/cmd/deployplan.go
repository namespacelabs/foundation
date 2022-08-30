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
	"namespacelabs.dev/foundation/internal/orchestration"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
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

		p := ops.NewPlan()
		if err := p.Add(plan.GetProgram().GetInvocation()...); err != nil {
			return fnerrors.New("failed to prepare plan: %w", err)
		}

		if orchestration.UseOrchestrator {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, plan.Environment.Name)
			if err != nil {
				return err
			}

			if _, err := orchestration.Deploy(ctx, env, plan); err != nil {
				return err
			}
		}

		return completeDeployment(ctx, serializedEnvironment{root, plan.Environment}, p, plan, opts)
	})

	return cmd
}

type serializedEnvironment struct {
	root *workspace.Root
	env  *schema.Environment
}

func (se serializedEnvironment) Workspace() *schema.Workspace { return se.root.Workspace }
func (se serializedEnvironment) DevHost() *schema.DevHost     { return se.root.DevHost }
func (se serializedEnvironment) Proto() *schema.Environment   { return se.env }
func (se serializedEnvironment) ErrorLocation() string        { return se.root.Abs() }

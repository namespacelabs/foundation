// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"io/ioutil"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
)

func NewDeployPlanCmd() *cobra.Command {
	var alsoWait = true

	cmd := &cobra.Command{
		Use:    "deploy-plan",
		Short:  "Deploys a previously serialized plan.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.Flags().BoolVar(&alsoWait, "wait", alsoWait, "Wait for the deployment after running.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		raw, err := ioutil.ReadFile(args[0])
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

		return completeDeployment(ctx, env, p, plan, alsoWait)
	})
}

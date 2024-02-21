// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"io/fs"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/module"
)

func NewDeployPlanCmd() *cobra.Command {
	var opts deployOpts
	var image, insecure bool

	cmd := &cobra.Command{
		Use:    "deploy-plan <path/to/plan> | <imageref>",
		Short:  "Deploys a previously serialized plan.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.Flags().BoolVar(&opts.alsoWait, "wait", true, "Wait for the deployment after running.")
	cmd.Flags().StringVar(&opts.outputPath, "output_to", "", "If set, a machine-readable output is emitted after successful deployment.")
	cmd.Flags().BoolVar(&image, "image", false, "If set to true, the argument represents an image.")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Access to the registry is insecure.")
	cmd.Flags().StringVar(&opts.manualReason, "reason", "", "Why was this deployment triggered.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		plan, err := loadPlan(ctx, image, insecure, args[0])
		if err != nil {
			return err
		}

		config, err := cfg.MakeConfigurationCompat(root, root.Workspace(), root.DevHost(), plan.Environment)
		if err != nil {
			return err
		}

		if deploy.RequireReason(config) && deployReason(opts) == "" {
			return fnerrors.New("--reason is required when deploying to environment %q", plan.Environment.Name)
		}

		env := serializedContext{root, config, plan.Environment}

		cluster, err := runtime.NamespaceFor(ctx, env)
		if err != nil {
			return err
		}

		return completeDeployment(ctx, env, cluster, plan, opts)
	})

	return cmd
}

type serializedContext struct {
	root   *parsing.Root
	config cfg.Configuration
	env    *schema.Environment
}

func (se serializedContext) Workspace() cfg.Workspace         { return se.root.Workspace() }
func (se serializedContext) Environment() *schema.Environment { return se.env }
func (se serializedContext) ErrorLocation() string            { return se.root.ErrorLocation() }
func (se serializedContext) Configuration() cfg.Configuration { return se.config }

func loadPlan(ctx context.Context, image, insecure bool, path string) (*schema.DeployPlan, error) {
	raw, err := loadPlanContents(ctx, image, insecure, path)
	if err != nil {
		return nil, fnerrors.New("failed to load %q: %w", path, err)
	}

	any := &anypb.Any{}
	if err := proto.Unmarshal(raw, any); err != nil {
		return nil, fnerrors.New("failed to unmarshal %q: %w", path, err)
	}

	plan := &schema.DeployPlan{}
	if err := any.UnmarshalTo(plan); err != nil {
		return nil, fnerrors.New("failed to unmarshal %q: %w", path, err)
	}

	return plan, nil
}

func loadPlanContents(ctx context.Context, image, insecure bool, path string) ([]byte, error) {
	if image {
		image, err := compute.GetValue(ctx, oci.ImageP(path, nil, oci.RegistryAccess{InsecureRegistry: insecure}))
		if err != nil {
			return nil, err
		}

		return fs.ReadFile(oci.ImageAsFS(image), "deployplan.binarypb")
	}

	return os.ReadFile(path)
}

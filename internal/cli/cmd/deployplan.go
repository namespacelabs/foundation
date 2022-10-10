// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"io"
	"io/fs"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewDeployPlanCmd() *cobra.Command {
	var opts deployOpts
	var image bool

	cmd := &cobra.Command{
		Use:    "deploy-plan <path/to/plan> | <imageref>",
		Short:  "Deploys a previously serialized plan.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.Flags().BoolVar(&opts.alsoWait, "wait", true, "Wait for the deployment after running.")
	cmd.Flags().StringVar(&opts.outputPath, "output_to", "", "If set, a machine-readable output is emitted after successful deployment.")
	cmd.Flags().BoolVar(&image, "image", false, "If set to true, the argument represents an image.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		plan, err := loadPlan(ctx, image, args[0])
		if err != nil {
			return err
		}

		config, err := planning.MakeConfigurationCompat(root, root.Workspace(), root.DevHost(), plan.Environment)
		if err != nil {
			return err
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
	root   *workspace.Root
	config planning.Configuration
	env    *schema.Environment
}

func (se serializedContext) Workspace() planning.Workspace         { return se.root.Workspace() }
func (se serializedContext) Environment() *schema.Environment      { return se.env }
func (se serializedContext) ErrorLocation() string                 { return se.root.ErrorLocation() }
func (se serializedContext) Configuration() planning.Configuration { return se.config }

func loadPlan(ctx context.Context, image bool, path string) (*schema.DeployPlan, error) {
	raw, err := loadPlanContents(ctx, image, path)
	if err != nil {
		return nil, fnerrors.New("failed to load %q: %w", path, err)
	}

	plan := &schema.DeployPlan{}
	if err := proto.Unmarshal(raw, plan); err != nil {
		return nil, fnerrors.New("failed to unmarshal %q: %w", path, err)
	}

	return plan, nil
}

func loadPlanContents(ctx context.Context, image bool, path string) ([]byte, error) {
	if image {
		image, err := compute.GetValue(ctx, oci.ImageP(path, nil, oci.ResolveOpts{}))
		if err != nil {
			return nil, err
		}

		fsys := tarfs.FS{TarStream: func() (io.ReadCloser, error) { return mutate.Extract(image), nil }}

		return fs.ReadFile(fsys, "deployplan.binarypb")
	}

	return os.ReadFile(path)
}

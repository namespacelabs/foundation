// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/kr/text"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workstation"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewPrepareCmd() *cobra.Command {
	var dontUpdateDevhost, force bool
	var envRef string = "dev"
	var contextName string
	var awsProfile string

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare the workspace for development or production.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envRef)
			if err != nil {
				return err
			}

			var prepares []workstation.PrepareFunc

			prepares = append(prepares, workstation.PrepareBuildkit())

			if contextName != "" {
				prepares = append(prepares, workstation.SetK8sContext(contextName))
			} else if env.Purpose() == schema.Environment_DEVELOPMENT {
				prepares = append(prepares, workstation.PrepareK3d("fn", true))
			}

			if awsProfile != "" {
				prepares = append(prepares, workstation.SetAWSProfile(awsProfile)) // XXX make provider configurable.
			}

			if env.Purpose() == schema.Environment_PRODUCTION {
				if awsProfile == "" {
					return fnerrors.UsageError("Please also specify `--aws_profile`.", "Preparing a production environment requires using AWS at the moment.")
				}
				if contextName == "" {
					return fnerrors.UsageError("Please also specify `--context`.", "Kubernetes context is required for preparing a production environment.")
				}

				prepares = append(prepares, workstation.PrepareAWSRegistry) // XXX make provider configurable.
			}

			prepares = append(prepares, workstation.PrepareIngress)

			stdout := console.Stdout(ctx)
			eg, wait := executor.New(ctx)

			eg.Go(func(ctx context.Context) error {
				r, err := workstation.Prepare(ctx, root, env, prepares...)
				if err != nil {
					return err
				}

				if !force && r.UpdateCount == 0 {
					fmt.Fprintln(stdout, "Configuration is up to date, nothing to do.")
					return nil
				}

				if !dontUpdateDevhost {
					return devhost.RewriteWith(ctx, root, env.Root().DevHost)
				}

				fmt.Fprintln(stdout, "Add the following to your devhost.textpb, in the root of your workspace:")
				fmt.Fprintln(stdout)
				fmt.Fprintln(
					text.NewIndentWriter(stdout, []byte("    ")),
					prototext.Format(env.Root().DevHost))
				fmt.Fprintln(stdout, "Or re-run with `fn prepare -w` for the file to be updated automatically.")

				return nil
			})

			eg.Go(func(ctx context.Context) error {
				hostPlatform := tools.Impl().HostPlatform()
				var required = []schema.PackageName{
					"namespacelabs.dev/foundation/std/sdk/buf/baseimg",
					"namespacelabs.dev/foundation/devworkflow/web",
					"namespacelabs.dev/foundation/std/dev/controller",
					"namespacelabs.dev/foundation/std/monitoring/grafana/tool",
					"namespacelabs.dev/foundation/std/monitoring/prometheus/tool",
					"namespacelabs.dev/foundation/std/secrets/kubernetes",
				}

				inputs := compute.Inputs()

				pl := workspace.NewPackageLoader(root)
				for _, pkg := range required {
					p, err := pl.LoadByName(ctx, pkg)
					if err != nil {
						return err
					}

					prepared, err := binary.PlanImage(ctx, p, env, true, &hostPlatform)
					if err != nil {
						return err
					}

					inputs = inputs.Computable(pkg.String(), prepared.Image)
				}

				// Pre-pull requires images.
				_, err := compute.Get(ctx, compute.Map(tasks.Action("prepare.prebuilts"), inputs,
					compute.Output{}, func(ctx context.Context, r compute.Resolved) (int, error) {
						return 0, nil
					}))
				return err
			})

			return wait()
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to prepare (as defined in the workspace).")
	cmd.Flags().BoolVar(&dontUpdateDevhost, "dont_update_devhost", dontUpdateDevhost, "If set to true, devhost.textpb will NOT be updated.")
	cmd.Flags().BoolVarP(&force, "force", "f", force, "Skip checking if the configuration is changing.")
	cmd.Flags().StringVar(&contextName, "context", "", "If set, configures Foundation to use the specific context.")
	cmd.Flags().StringVar(&awsProfile, "aws_profile", awsProfile, "Configures the specified AWS configuration profile.")
	return cmd
}
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/kr/text"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var deprecatedConfigs = []string{
	"type.googleapis.com/foundation.build.buildkit.Configuration",
}

func NewPrepareCmd() *cobra.Command {
	var dontUpdateDevhost, force bool
	var contextName string
	var awsProfile string

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare the workspace for development or production.",
		Args:  cobra.NoArgs,
	}

	cmd.Flags().BoolVar(&dontUpdateDevhost, "dont_update_devhost", dontUpdateDevhost, "If set to true, devhost.textpb will NOT be updated.")
	cmd.Flags().BoolVarP(&force, "force", "f", force, "Skip checking if the configuration is changing.")
	cmd.Flags().StringVar(&contextName, "context", "", "If set, configures Foundation to use the specific context.")
	cmd.Flags().StringVar(&awsProfile, "aws_profile", awsProfile, "Configures the specified AWS configuration profile.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		var prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
		prepares = append(prepares, prepare.PrepareBuildkit(env))

		var k8sconfig compute.Computable[*kubernetes.HostConfig]
		if contextName != "" {
			k8sconfig = prepare.PrepareExistingK8s(contextName, env)
		} else if env.Purpose() == schema.Environment_DEVELOPMENT {
			k8sconfig = prepare.PrepareK3d("fn", env)
		}

		prepares = append(prepares, compute.Map(
			tasks.Action("prepare.map-k8s"),
			compute.Inputs().Computable("k8sconfig", k8sconfig),
			compute.Output{NotCacheable: true},
			func(ctx context.Context, deps compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
				k8sconfigval := compute.MustGetDepValue(deps, k8sconfig, "k8sconfig")
				var confs []*schema.DevHost_ConfigureEnvironment
				registry := k8sconfigval.Registry()
				if registry != nil {
					c, err := devhost.MakeConfiguration(registry)
					if err != nil {
						return nil, err
					}
					c.Purpose = schema.Environment_DEVELOPMENT
					confs = append(confs, c)
				}
				hostEnv := k8sconfigval.ClientHostEnv()
				if hostEnv != nil {
					c, err := devhost.MakeConfiguration(hostEnv)
					if err != nil {
						return nil, err
					}
					c.Purpose = env.Proto().GetPurpose()
					c.Runtime = "kubernetes"
					confs = append(confs, c)
				}
				return confs, nil
			}))

		if awsProfile != "" {
			prepares = append(prepares, prepare.PrepareAWSProfile(awsProfile, env)) // XXX make provider configurable.
		}

		if env.Purpose() == schema.Environment_PRODUCTION {
			if awsProfile == "" {
				return fnerrors.UsageError("Please also specify `--aws_profile`.", "Preparing a production environment requires using AWS at the moment.")
			}
			if contextName == "" {
				return fnerrors.UsageError("Please also specify `--context`.", "Kubernetes context is required for preparing a production environment.")
			}

			prepares = append(prepares, prepare.PrepareAWSRegistry(env)) // XXX make provider configurable.
		}

		prepares = append(prepares, prepare.PrepareIngress(env, k8sconfig))

		var prebuilts = []schema.PackageName{
			"namespacelabs.dev/foundation/std/sdk/buf/baseimg",
			"namespacelabs.dev/foundation/devworkflow/web",
			"namespacelabs.dev/foundation/std/dev/controller",
			"namespacelabs.dev/foundation/std/monitoring/grafana/tool",
			"namespacelabs.dev/foundation/std/monitoring/prometheus/tool",
			"namespacelabs.dev/foundation/std/secrets/kubernetes",
		}

		preparedPrebuilts := prepare.DownloadPrebuilts(env, workspace.NewPackageLoader(env.Root()), prebuilts)
		prepares = append(prepares, compute.Map(
			tasks.Action("prepare.map-prebuilts"),
			compute.Inputs().Computable("preparedPrebuilts", preparedPrebuilts),
			compute.Output{NotCacheable: true},
			func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
				return nil, nil
			}))

		prepareAll := compute.Collect(tasks.Action("prepare.collect-all"), prepares...)
		results, err := compute.GetValue(ctx, prepareAll)
		if err != nil {
			return err
		}

		var confs [][]*schema.DevHost_ConfigureEnvironment
		for _, r := range results {
			confs = append(confs, r.Value)
		}

		stdout := console.Stdout(ctx)

		updateCount, err := devHostUpdates(ctx, env.Root(), confs)
		if err != nil {
			return err
		}

		if !force && updateCount == 0 {
			fmt.Fprintln(stdout, "Configuration is up to date, nothing to do.")
			return nil
		}

		if !dontUpdateDevhost {
			return devhost.RewriteWith(ctx, env.Root(), env.DevHost())
		}

		fmt.Fprintln(stdout, "Add the following to your devhost.textpb, in the root of your workspace:")
		fmt.Fprintln(stdout)
		fmt.Fprintln(
			text.NewIndentWriter(stdout, []byte("    ")),
			prototext.Format(env.DevHost()))
		fmt.Fprintln(stdout, "Or re-run with `fn prepare -w` for the file to be updated automatically.")

		return nil
	})
}

func devHostUpdates(ctx context.Context, root *workspace.Root, confs [][]*schema.DevHost_ConfigureEnvironment) (int, error) {
	var updateCount int
	for _, conf := range confs {
		if len(conf) == 0 {
			continue
		}

		updated, was := devhost.Update(root, conf...)
		if was {
			updateCount++
		}

		// Make sure that the subsequent calls observe an up to date configuration.
		// XXX this is not right, Root() should be immutable.
		root.DevHost = updated
	}

	// Remove deprecated bits.
	for k, u := range root.DevHost.Configure {
		var without []*anypb.Any

		for _, cfg := range u.Configuration {
			if !slices.Contains(deprecatedConfigs, cfg.TypeUrl) {
				without = append(without, cfg)
			} else {
				updateCount++
			}
		}

		if len(without) == 0 {
			root.DevHost.Configure[k] = nil // Mark for removal.
		} else {
			u.Configuration = without
		}
	}

	k := 0
	for {
		if k >= len(root.DevHost.Configure) {
			break
		}

		if root.DevHost.Configure[k] == nil {
			root.DevHost.Configure = append(root.DevHost.Configure[:k], root.DevHost.Configure[k+1:]...)
			updateCount++
		} else {
			k++
		}
	}

	return updateCount, nil
}

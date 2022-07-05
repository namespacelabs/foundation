// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var deprecatedConfigs = []string{
	"type.googleapis.com/foundation.build.buildkit.Configuration",
}

func NewPrepareCmd() *cobra.Command {
	var contextName string
	var awsProfile string
	var envRef string

	// The subcommand `eks` does all of the work done by the parent command in addition to
	// writing the host configuration for the EKS cluster.
	eksCmd := &cobra.Command{
		Use:   "eks [clustername]",
		Short: "Prepares the Elastic Kubernetes Service host config for production.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fnerrors.UserError(nil, "%q is a required argument, run %q to proceed", "[clustername]", "eks [clustername]")
			}
			return nil
		},
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envRef)
			if err != nil {
				return err
			}
			if env.Purpose() == schema.Environment_DEVELOPMENT {
				return fnerrors.UsageError("Use `--env=prod`.", "eks is not supported yet in development mode.")
			}
			if contextName != "" {
				return fnerrors.UsageError("Remove `--context=<name>`.", "--context was provided but is not used in the eks integration.")
			}
			eksClusterName := args[0]

			wp := workspacePrepare{
				env:            env,
				awsProfile:     awsProfile,
				eksClusterName: eksClusterName,
			}
			prepares, err := wp.makePrepareComputables(ctx)
			if err != nil {
				return err
			}
			prepares = append(prepares, prepare.PrepareEksCluster(eksClusterName, env))
			return wp.collectPreparesAndUpdateDevhost(ctx, prepares)
		}),
	}

	rootCmd := &cobra.Command{
		Use:   "prepare local",
		Short: "Prepares the local workspace for development or production.",
		Long: "Prepares the local workspace for development or production.\n\n" +
			"This command will download, create, and run Buildkit and Kubernetes\n" +
			"orchestration containers (conditional on development or production),\n" +
			"in addition to downloading and caching required pre-built images.\n" +
			"Developers will typically run this command only after initializing\n" +
			"the workspace, and it's not a part of the normal refresh-edit\n" +
			"workspace lifecycle.",
		Args: func(cmd *cobra.Command, args []string) error {
			expectedArg := "local"
			expectedCmd := "ns prepare local"
			if len(args) < 1 {
				return fnerrors.UserError(nil, "%q is a required argument, run %q to proceed", expectedArg, expectedCmd)
			}
			if args[0] == "aws" {
				return fnerrors.UserError(nil, "aws is not a supported target. Use local to setup a local development environment, or eks to setup a EKS-based environment.")
			}
			if args[0] != "local" {
				return fnerrors.UserError(nil, "%q is a required argument, run %q to proceed", expectedArg, expectedCmd)
			}
			return nil
		},
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envRef)
			if err != nil {
				return err
			}

			if env.Purpose() == schema.Environment_PRODUCTION && contextName == "" {
				return fnerrors.UsageError("Please also specify `--context`.",
					"Kubernetes context is required for preparing a production environment.")
			}

			wp := workspacePrepare{
				env:         env,
				awsProfile:  awsProfile,
				contextName: contextName,
			}
			prepares, err := wp.makePrepareComputables(ctx)
			if err != nil {
				return err
			}
			return wp.collectPreparesAndUpdateDevhost(ctx, prepares)
		}),
	}
	rootCmd.AddCommand(eksCmd)
	rootCmd.PersistentFlags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")
	rootCmd.PersistentFlags().StringVar(&contextName, "context", "", "If set, configures Namespace to use the specific context.")
	rootCmd.PersistentFlags().StringVar(&awsProfile, "aws_profile", awsProfile, "Configures the specified AWS configuration profile.")

	return rootCmd
}

type workspacePrepare struct {
	env            provision.Env
	awsProfile     string
	contextName    string
	eksClusterName string
}

func (p *workspacePrepare) PrepareWorkspace(ctx context.Context) error {
	prepares, err := p.makePrepareComputables(ctx)
	if err != nil {
		return err
	}
	return p.collectPreparesAndUpdateDevhost(ctx, prepares)
}

func (p *workspacePrepare) prepareK8s(ctx context.Context) compute.Computable[*client.HostConfig] {
	if p.env.Purpose() == schema.Environment_DEVELOPMENT {
		return prepare.PrepareK3d("fn", p.env)
	} else if p.env.Purpose() == schema.Environment_PRODUCTION {
		if p.contextName != "" {
			return prepare.PrepareExistingK8s(p.env,
				prepare.WithK8sContextName(p.contextName))
		} else if p.eksClusterName != "" {
			return prepare.PrepareExistingK8s(p.env)
		}
	}
	return nil
}

func (p *workspacePrepare) makePrepareComputables(ctx context.Context) ([]compute.Computable[[]*schema.DevHost_ConfigureEnvironment], error) {
	var prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
	prepares = append(prepares, prepare.PrepareBuildkit(p.env))

	k8sconfig := p.prepareK8s(ctx)
	if k8sconfig != nil {
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
					c.Purpose = p.env.Proto().GetPurpose()
					c.Runtime = "kubernetes"
					confs = append(confs, c)
				}
				return confs, nil
			}))
	}

	if p.awsProfile != "" {
		prepares = append(prepares, prepare.PrepareAWSProfile(p.awsProfile, p.env)) // XXX make provider configurable.
	}

	if p.env.Purpose() == schema.Environment_PRODUCTION {
		if p.awsProfile == "" {
			return nil, fnerrors.UsageError("Please also specify `--aws_profile`.",
				"Preparing a production environment requires using AWS at the moment.")
		}
		// The context name is required if we are not on an EKS cluster.
		if p.contextName == "" && p.eksClusterName == "" {
			return nil, fnerrors.UsageError("Please also specify `--context`.",
				"Kubernetes context is required for preparing a production environment.")
		}

		prepares = append(prepares, prepare.PrepareAWSRegistry(p.env)) // XXX make provider configurable.
	}

	if k8sconfig != nil {
		prepares = append(prepares, prepare.PrepareIngress(p.env, k8sconfig))
	}

	var prebuilts = []schema.PackageName{
		"namespacelabs.dev/foundation/devworkflow/web",
		"namespacelabs.dev/foundation/std/dev/controller",
		"namespacelabs.dev/foundation/std/monitoring/grafana/tool",
		"namespacelabs.dev/foundation/std/monitoring/prometheus/tool",
		"namespacelabs.dev/foundation/std/secrets/kubernetes",
	}

	preparedPrebuilts := prepare.DownloadPrebuilts(p.env, workspace.NewPackageLoader(p.env.Root()), prebuilts)
	prepares = append(prepares, compute.Map(
		tasks.Action("prepare.map-prebuilts"),
		compute.Inputs().Computable("preparedPrebuilts", preparedPrebuilts),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			return nil, nil
		}))
	return prepares, nil
}

func (p *workspacePrepare) collectPreparesAndUpdateDevhost(ctx context.Context, prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]) error {
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

	updateCount, err := devHostUpdates(ctx, p.env.Root(), confs)
	if err != nil {
		return err
	}

	if updateCount == 0 {
		fmt.Fprintln(stdout, "Configuration is up to date, nothing to do.")
		return nil
	}
	return devhost.RewriteWith(ctx, p.env.Root(), p.env.DevHost())
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

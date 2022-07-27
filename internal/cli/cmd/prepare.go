// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
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
	var clusterName string

	// The subcommand `eks` does all of the work done by the parent command in addition to
	// writing the host configuration for the EKS cluster.
	eksCmd := &cobra.Command{
		Use:   "eks",
		Short: "Prepares the Elastic Kubernetes Service host config for production.",
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

			prepares := baseline(env)

			var aws []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
			aws = append(aws, prepare.PrepareAWSProfile(env, awsProfile))
			aws = append(aws, prepare.PrepareEksCluster(env, clusterName))
			prepares = append(prepares, aws...)
			prepares = append(prepares, prepare.PrepareIngress(env, instantiateKube(env, aws)))
			return collectPreparesAndUpdateDevhost(ctx, env, prepares)
		}),
	}

	eksCmd.Flags().StringVar(&clusterName, "cluster", "", "The name of the cluster we're configuring.")
	eksCmd.Flags().StringVar(&awsProfile, "aws_profile", awsProfile, "Configures the specified AWS configuration profile.")

	_ = cobra.MarkFlagRequired(eksCmd.Flags(), "cluster")
	_ = cobra.MarkFlagRequired(eksCmd.Flags(), "aws_profile")

	localCmd := &cobra.Command{
		Use:   "local",
		Short: "Prepares the local workspace for development or production.",

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

			prepares := baseline(env)

			k8sconfig := prepareK8s(ctx, env, contextName)
			prepares = append(prepares, localK8sConfiguration(env, k8sconfig))
			prepares = append(prepares, prepare.PrepareIngressFromHostConfig(env, k8sconfig))

			return collectPreparesAndUpdateDevhost(ctx, env, prepares)
		}),
	}

	localCmd.Flags().StringVar(&contextName, "context", "", "If set, configures Namespace to use the specific context.")

	rootCmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepares the local workspace for development or production.",
		Long: "Prepares the local workspace for development or production.\n\n" +
			"This command will download, create, and run Buildkit and Kubernetes\n" +
			"orchestration containers (conditional on development or production),\n" +
			"in addition to downloading and caching required pre-built images.\n" +
			"Developers will typically run this command only after initializing\n" +
			"the workspace, and it's not a part of the normal refresh-edit\n" +
			"workspace lifecycle.",
		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return fnerrors.UsageError("For example, you may call `ns prepare local` to configure a local development environment.",
				"One of `local` or `eks` is required.")
		}),
	}

	rootCmd.AddCommand(eksCmd)
	rootCmd.AddCommand(localCmd)

	rootCmd.PersistentFlags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")

	return rootCmd
}

func instantiateKube(env ops.Environment, confs []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]) compute.Computable[kubernetes.Unbound] {
	return compute.Map(tasks.Action("prepare.kubernetes"),
		compute.Inputs().Computable("conf", compute.Transform(compute.Collect(tasks.Action("prepare.kubernetes.configs"), confs...),
			func(ctx context.Context, computed []compute.ResultWithTimestamp[[]*schema.DevHost_ConfigureEnvironment]) ([]*schema.DevHost_ConfigureEnvironment, error) {
				var result []*schema.DevHost_ConfigureEnvironment
				for _, conf := range computed {
					result = append(result, conf.Value...)
				}
				return result, nil
			})),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (kubernetes.Unbound, error) {
			computed, _ := compute.GetDepWithType[[]*schema.DevHost_ConfigureEnvironment](r, "conf")

			return kubernetes.New(ctx, env.Proto(), &schema.DevHost{Configure: computed.Value}, devhost.ByEnvironment(env.Proto()))
		})
}

func prepareK8s(ctx context.Context, env provision.Env, contextName string) compute.Computable[*client.HostConfig] {
	if contextName != "" {
		return prepare.PrepareExistingK8s(env, prepare.WithK8sContextName(contextName))
	}

	return prepare.PrepareK3d("fn", env)
}

func localK8sConfiguration(env provision.Env, hostConfig compute.Computable[*client.HostConfig]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Transform(hostConfig, func(ctx context.Context, k8sconfigval *client.HostConfig) ([]*schema.DevHost_ConfigureEnvironment, error) {
		var messages []proto.Message

		registry := k8sconfigval.Registry()
		if registry != nil {
			messages = append(messages, registry)
		}

		hostEnv := k8sconfigval.ClientHostEnv()
		if hostEnv != nil {
			messages = append(messages, hostEnv)
		}

		c, err := devhost.MakeConfiguration(messages...)
		if err != nil {
			return nil, err
		}
		c.Name = env.Proto().GetName()
		c.Runtime = "kubernetes"

		var confs []*schema.DevHost_ConfigureEnvironment
		confs = append(confs, c)
		return confs, nil
	})
}

func baseline(env provision.Env) []compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	var prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
	prepares = append(prepares, prepare.PrepareBuildkit(env))
	prepares = append(prepares, prebuilts(env)...)
	return prepares
}

func prebuilts(env provision.Env) []compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	var prebuilts = []schema.PackageName{
		"namespacelabs.dev/foundation/devworkflow/web",
		"namespacelabs.dev/foundation/std/dev/controller",
		"namespacelabs.dev/foundation/std/monitoring/grafana/tool",
		"namespacelabs.dev/foundation/std/monitoring/prometheus/tool",
		"namespacelabs.dev/foundation/std/secrets/kubernetes",
	}

	preparedPrebuilts := prepare.DownloadPrebuilts(env, workspace.NewPackageLoader(env.Root()), prebuilts)

	var prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
	prepares = append(prepares, compute.Map(
		tasks.Action("prepare.map-prebuilts"),
		compute.Inputs().Computable("preparedPrebuilts", preparedPrebuilts),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			return nil, nil
		}))
	return prepares
}

func collectPreparesAndUpdateDevhost(ctx context.Context, env provision.Env, prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]) error {
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

	if updateCount == 0 {
		fmt.Fprintln(stdout, "Configuration is up to date, nothing to do.")
		return nil
	}

	return devhost.RewriteWith(ctx, env.Root(), env.DevHost())
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

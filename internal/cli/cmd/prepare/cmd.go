// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

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
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var deprecatedConfigs = []string{
	"type.googleapis.com/foundation.build.buildkit.Configuration",
}

var envRef string

func NewPrepareCmd() *cobra.Command {
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

	rootCmd.AddCommand(newEksCmd())
	rootCmd.AddCommand(newLocalCmd())
	rootCmd.AddCommand(newNewClusterCmd())

	rootCmd.PersistentFlags().StringVar(&envRef, "env", "dev", "The environment to access (as defined in the workspace).")

	return rootCmd
}

func instantiateKube(env planning.Context, confs []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]) compute.Computable[kubernetes.Cluster] {
	return compute.Map(tasks.Action("prepare.kubernetes"),
		compute.Inputs().Computable("conf", compute.Transform("parse results", compute.Collect(tasks.Action("prepare.kubernetes.configs"), confs...),
			func(ctx context.Context, computed []compute.ResultWithTimestamp[[]*schema.DevHost_ConfigureEnvironment]) ([]*schema.DevHost_ConfigureEnvironment, error) {
				var result []*schema.DevHost_ConfigureEnvironment
				for _, conf := range computed {
					result = append(result, conf.Value...)
				}
				return result, nil
			})),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (kubernetes.Cluster, error) {
			computed, _ := compute.GetDepWithType[[]*schema.DevHost_ConfigureEnvironment](r, "conf")

			var merged []*anypb.Any
			for _, m := range computed.Value {
				merged = append(merged, m.Configuration...)
			}

			return kubernetes.NewCluster(ctx, planning.MakeConfigurationWith(env.Environment().Name, planning.ConfigurationSlice{Configuration: merged}))
		})
}

func baseline(env planning.Context) []compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	var prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]
	prepares = append(prepares, prepare.PrepareBuildkit(env))
	prepares = append(prepares, prebuilts(env)...)
	return prepares
}

func prebuilts(env planning.Context) []compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	var prebuilts = []schema.PackageName{
		"namespacelabs.dev/foundation/devworkflow/web",
		"namespacelabs.dev/foundation/std/dev/controller",
		"namespacelabs.dev/foundation/std/monitoring/grafana/tool",
		"namespacelabs.dev/foundation/std/monitoring/prometheus/tool",
		"namespacelabs.dev/foundation/std/secrets/kubernetes",
	}

	preparedPrebuilts := prepare.DownloadPrebuilts(env, workspace.NewPackageLoader(env), prebuilts)

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

func collectPreparesAndUpdateDevhost(ctx context.Context, root *workspace.Root, prepares []compute.Computable[[]*schema.DevHost_ConfigureEnvironment]) error {
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

	updateCount, err := devHostUpdates(ctx, root, confs)
	if err != nil {
		return err
	}

	if updateCount == 0 {
		fmt.Fprintln(stdout, "Configuration is up to date, nothing to do.")
		return nil
	}

	return devhost.RewriteWith(ctx, root.ReadWriteFS(), devhost.DevHostFilename, root.LoadedDevHost)
}

func devHostUpdates(ctx context.Context, root *workspace.Root, confs [][]*schema.DevHost_ConfigureEnvironment) (int, error) {
	var updateCount int
	for _, conf := range confs {
		if len(conf) == 0 {
			continue
		}

		updated, was := devhost.Update(root.LoadedDevHost, conf...)
		if was {
			updateCount++
		}

		// Make sure that the subsequent calls observe an up to date configuration.
		// XXX this is not right, Root() should be immutable.
		root.LoadedDevHost = updated
	}

	// Remove deprecated bits.
	for k, u := range root.LoadedDevHost.Configure {
		var without []*anypb.Any

		for _, cfg := range u.Configuration {
			if !slices.Contains(deprecatedConfigs, cfg.TypeUrl) {
				without = append(without, cfg)
			} else {
				updateCount++
			}
		}

		if len(without) == 0 {
			root.LoadedDevHost.Configure[k] = nil // Mark for removal.
		} else {
			u.Configuration = without
		}
	}

	k := 0
	for {
		if k >= len(root.LoadedDevHost.Configure) {
			break
		}

		if root.LoadedDevHost.Configure[k] == nil {
			root.LoadedDevHost.Configure = append(root.LoadedDevHost.Configure[:k], root.LoadedDevHost.Configure[k+1:]...)
			updateCount++
		} else {
			k++
		}
	}

	return updateCount, nil
}

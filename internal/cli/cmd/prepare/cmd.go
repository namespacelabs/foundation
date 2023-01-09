// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/renderwait"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/prepare"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
)

var deprecatedConfigs = []string{
	"type.googleapis.com/foundation.build.buildkit.Configuration",
}

var (
	envRef           string
	isCreateEnv      bool   = false
	createEnvPurpose string = "DEVELOPMENT"
)

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
				"One of `local`, `existing`, `eks` or `gke` is required.")
		}),
	}

	rootCmd.AddCommand(newEksCmd())
	rootCmd.AddCommand(newGkeCmd())
	rootCmd.AddCommand(newLocalCmd())
	rootCmd.AddCommand(newExistingCmd())
	rootCmd.AddCommand(newNewClusterCmd())
	rootCmd.AddCommand(newNewBuildClusterCmd())

	rootCmd.PersistentFlags().StringVar(&envRef, "env", "dev", "The environment to access.")
	rootCmd.PersistentFlags().BoolVar(&isCreateEnv, "create_env", isCreateEnv, "Create the environment with a defined parameters and writes it into the workspace file if it is not exists yet.")
	rootCmd.PersistentFlags().StringVar(&createEnvPurpose, "env_purpose", createEnvPurpose, "The purpose the newly create environment")

	return rootCmd
}

func NewPrepareIngressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "prepare-ingress",
		Hidden: true,
	}

	env := cmd.Flags().String("env", "dev", "The environment to prepare.")

	return fncobra.With(cmd, func(ctx context.Context) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		env, err := cfg.LoadContext(root, *env)
		if err != nil {
			return err
		}

		kube, err := kubernetes.ConnectToCluster(ctx, env.Configuration())
		if err != nil {
			return err
		}

		return prepare.PrepareIngressInKube(ctx, env, kube)
	})
}

func NewPrepareBuildClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "prepare-build-cluster",
		Hidden: true,
	}

	env := cmd.Flags().String("env", "dev", "The environment to prepare.")

	return fncobra.With(cmd, func(ctx context.Context) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		env, err := cfg.LoadContext(root, *env)
		if err != nil {
			return err
		}

		kube, err := kubernetes.ConnectToCluster(ctx, env.Configuration())
		if err != nil {
			return err
		}

		jobdef, err := prepare.PrepareBuildCluster(ctx, env, kube)
		if err != nil {
			return err
		}

		buildkitConfiguration, err := anypb.New(&buildkit.Overrides{
			ColocatedBuildCluster: &buildkit.ColocatedBuildCluster{
				Namespace:         jobdef.Namespace.Name,
				MatchingPodLabels: jobdef.MatchingLabels,
				TargetPort:        jobdef.Service.Spec.Ports[0].Port,
			},
		})
		if err != nil {
			return err
		}

		updated, was := devhost.Update(root.LoadedDevHost, &schema.DevHost_ConfigureEnvironment{
			Name:          env.Environment().Name,
			Configuration: []*anypb.Any{buildkitConfiguration},
		})

		if was {
			if err := devhost.RewriteWith(ctx, root.ReadWriteFS(), devhost.DevHostFilename, updated); err != nil {
				return err
			}
		}

		return nil
	})
}

func parseCreateEnvArgs() (*schema.Workspace_EnvironmentSpec, error) {
	purpose, ok := schema.Environment_Purpose_value[strings.ToUpper(createEnvPurpose)]
	if !ok || purpose == 0 {
		return nil, fnerrors.New("no such environment purpose %q", createEnvPurpose)
	}
	env := &schema.Workspace_EnvironmentSpec{
		Name:    envRef,
		Runtime: "kubernetes",
		Purpose: schema.Environment_Purpose(purpose),
	}

	return env, nil
}

func updateWorkspaceEnvironment(ctx context.Context, envRef string) error {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	newEnv, err := parseCreateEnvArgs()
	if err != nil {
		return err
	}

	ws := root.EditableWorkspace().WithSetEnvironment(newEnv)
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), root.ReadWriteFS(), ws.DefinitionFile(), func(w io.Writer) error {
		return ws.FormatTo(w)
	})
}

func runPrepare(callback func(context.Context, cfg.Context) ([]prepare.Stage, error)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return fncobra.RunE(func(ctx context.Context, args []string) error {
			if isCreateEnv {
				err := updateWorkspaceEnvironment(ctx, envRef)
				if err != nil {
					return err
				}
			}
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := cfg.LoadContext(root, envRef)
			if err != nil {
				return err
			}

			prepared, err := callback(ctx, env)
			if err != nil {
				return err
			}

			rwb := renderwait.NewBlock(ctx, "prepare")
			eg := executor.New(ctx, "prepare")

			clusterStages := []prepare.ClusterStage{
				{Pre: func(ch chan *orchestration.Event) {
					ch <- &orchestration.Event{
						Category:      "Preparing cluster",
						ResourceId:    "connect-to-cluster",
						Scope:         "Connect to cluster", // XXX remove soon.
						ResourceLabel: "Connect to cluster",
						Stage:         orchestration.Event_WAITING,
					}
				}},
				prepare.Ingress(),
				prepare.Orchestrator(),
			}

			for _, stage := range prepared {
				stage.Pre(rwb.Ch())
			}

			for _, stage := range clusterStages {
				stage.Pre(rwb.Ch())
			}

			eg.Go(func(ctx context.Context) error {
				merged := &schema.DevHost_ConfigureEnvironment{}
				for _, stage := range prepared {
					conf, err := stage.Run(ctx, env, rwb.Ch())
					if err != nil {
						return err
					}
					if conf != nil {
						merged.Configuration = append(merged.Configuration, conf.Configuration...)
					}
				}

				eg.Go(func(ctx context.Context) error {
					return collectPreparesAndUpdateDevhost(ctx, root, env, merged)
				})

				eg.Go(func(ctx context.Context) error {
					kube, err := prepare.InstantiateKube(ctx, env, merged)
					if err != nil {
						return err
					}

					rwb.Ch() <- &orchestration.Event{
						ResourceId: "connect-to-cluster",
						Ready:      orchestration.Event_READY,
						Stage:      orchestration.Event_DONE,
					}

					for _, stage := range clusterStages {
						stage := stage // Close stage.
						if stage.Run == nil {
							continue
						}

						eg.Go(func(ctx context.Context) error {
							if err := stage.Run(ctx, env, merged, kube, rwb.Ch()); err != nil {
								return err
							}

							stage.Post(rwb.Ch())
							return nil
						})
					}

					return nil
				})

				return nil
			})

			waitErr := eg.Wait()
			close(rwb.Ch())
			rwbErr := rwb.Wait(ctx)

			if waitErr != nil {
				return waitErr
			}

			if rwbErr != nil {
				return rwbErr
			}

			fmt.Fprintf(console.TypedOutput(ctx, "prepare", console.CatOutputUs), "\n%s\n", successMessage(env, cmd))
			return nil
		})(cmd, args)
	}
}

func collectPreparesAndUpdateDevhost(ctx context.Context, root *parsing.Root, env cfg.Context, results *schema.DevHost_ConfigureEnvironment) error {
	cloned := protos.Clone(results)
	cloned.Name = env.Environment().Name

	updateCount, err := devHostUpdates(ctx, root, cloned)
	if err != nil {
		return err
	}

	if updateCount == 0 {
		return nil
	}

	if err := devhost.RewriteWith(ctx, root.ReadWriteFS(), devhost.DevHostFilename, root.LoadedDevHost); err != nil {
		return err
	}

	return nil
}

func successMessage(env cfg.Context, cmd *cobra.Command) string {
	var b strings.Builder

	var purpose string
	switch env.Environment().Purpose {
	case schema.Environment_DEVELOPMENT:
		purpose = "development"
	case schema.Environment_PRODUCTION:
		purpose = "production"
	case schema.Environment_TESTING:
		purpose = "testing"
	}

	parts := strings.SplitN(cmd.Use, " ", 2)
	// Only consider the command (ignore flags from use)
	switch parts[0] {
	case "local":
		purpose = fmt.Sprintf("local %s", purpose)
	case "eks":
		purpose = fmt.Sprintf("AWS EKS %s", purpose)
	case "new-cluster":
		purpose = fmt.Sprintf("Namespace Cloud %s", purpose)
	case "existing":
		purpose = fmt.Sprintf("%s using your existing environment", purpose)
	}

	b.WriteString(fmt.Sprintf(" 🎉 %q is now configured for %s.\n\n", env.Workspace().ModuleName(), purpose))

	var envParam string
	if env.Environment().Name != "dev" {
		envParam = fmt.Sprintf(" --env=%s", env.Environment().Name)
	}

	b.WriteString(fmt.Sprintf(" You can now run servers using `ns dev%s`, tests using `ns test%s`, and more.\n", envParam, envParam))
	b.WriteString("\n Find out more at https://namespace.so/docs.")

	return b.String()
}

func devHostUpdates(ctx context.Context, root *parsing.Root, confs ...*schema.DevHost_ConfigureEnvironment) (int, error) {
	var updateCount int
	updated, was := devhost.Update(root.LoadedDevHost, confs...)
	if was {
		updateCount++
	}

	// Make sure that the subsequent calls observe an up to date configuration.
	// XXX this is not right, Root() should be immutable.
	root.LoadedDevHost = updated

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

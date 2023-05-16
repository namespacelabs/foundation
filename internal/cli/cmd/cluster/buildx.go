// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/buildx/store"
	"github.com/docker/buildx/store/storeutil"
	"github.com/docker/buildx/util/dockerutil"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/muesli/reflow/wordwrap"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const defaultBuilder = "nsc-remote"

func newSetupBuildxCmd(cmdName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdName,
		Short: "Setup buildx in the current machine, to use Namespace Remote builders.",
	}

	name := cmd.Flags().String("name", defaultBuilder, "The name of the builder we setup.")
	use := cmd.Flags().Bool("use", false, "If true, changes the current builder to nsc-remote.")
	background := cmd.Flags().Bool("background", false, "If true, runs the proxies in the background.")
	createAtStartup := cmd.Flags().Bool("create_at_startup", false, "If true, creates the build clusters eagerly.")
	stateDir := cmd.Flags().String("state", "", "If set, stores the proxy sockets in this directory.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		dockerCli, err := command.NewDockerCli()
		if err != nil {
			return err
		}

		if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
			return err
		}

		eg := executor.New(ctx, "proxies")

		available, err := determineAvailable(ctx)
		if err != nil {
			return err
		}

		if len(available) == 0 {
			return rpcerrors.Errorf(codes.Internal, "no builders available")
		}

		state, err := ensureStateDir(*stateDir, "buildkit", "proxy")
		if err != nil {
			return err
		}

		var md buildxMetadata
		for _, p := range available {
			sockPath := filepath.Join(state, fmt.Sprintf("%s.sock", p))
			md.Instances = append(md.Instances, buildxInstanceMetadata{
				Platform:   p,
				SocketPath: sockPath,
			})
		}

		if err := files.WriteJson(filepath.Join(state, "metadata.json"), md, 0644); err != nil {
			return err
		}

		var instances []*BuildClusterInstance
		for _, p := range md.Instances {
			// Always create one, in case it's needed below. This instance has a zero-ish cost if we never call NewConn.
			instance := NewBuildClusterInstance0(p.Platform)
			instances = append(instances, instance)

			if *background {
				if _, err := startBackgroundProxy(ctx, p, *createAtStartup); err != nil {
					return err
				}
			} else {
				bp, err := instance.runBuildProxy(ctx, p.SocketPath)
				if err != nil {
					return err
				}

				defer os.Remove(p.SocketPath)

				eg.Go(func(ctx context.Context) error {
					<-ctx.Done()
					return bp.Cleanup()
				})

				eg.Go(func(ctx context.Context) error {
					return bp.Serve(ctx)
				})
			}
		}

		if *createAtStartup {
			eg := executor.New(ctx, "startup")

			for _, p := range instances {
				p := p // Close p
				eg.Go(func(ctx context.Context) error {
					_, err := p.NewConn(ctx)
					return err
				})
			}

			if err := eg.Wait(); err != nil {
				return err
			}
		}

		if err := wireBuildx(dockerCli, *name, *use, md); err != nil {
			return multierr.New(err, eg.CancelAndWait())
		}

		if *background {
			return nil
		}

		console.SetStickyContent(ctx, "build", banner(ctx, *name, *use, available))

		if err := eg.Wait(); err != nil {
			return err
		}

		if err := withStore(dockerCli, func(txn *store.Txn) error {
			return txn.Remove(*name)
		}); err != nil {
			return err
		}

		return nil
	})

	return cmd
}

func ensureStateDir(specified, dir, suffix string) (string, error) {
	if specified == "" {
		return dirs.CreateUserTempDir(dir, suffix)
	}

	s, err := filepath.Abs(specified)
	if err != nil {
		return "", err
	}

	return s, nil
}

type buildxMetadata struct {
	Instances []buildxInstanceMetadata `json:"instances"`
}

type buildxInstanceMetadata struct {
	Platform   api.BuildPlatform `json:"build_platform"`
	SocketPath string            `json:"socket_path"`
}

func wireBuildx(dockerCli *command.DockerCli, name string, use bool, md buildxMetadata) error {
	return withStore(dockerCli, func(txn *store.Txn) error {
		ng, err := txn.NodeGroupByName(name)
		if err != nil {
			if !os.IsNotExist(errors.Cause(err)) {
				return err
			}
		}

		const driver = "remote"

		if ng == nil {
			ng = &store.NodeGroup{
				Name:   name,
				Driver: driver,
			}
		}

		for _, p := range md.Instances {
			var platforms []string
			if p.Platform == "arm64" {
				platforms = []string{"linux/arm64"}
			}

			if err := ng.Update(string(p.Platform), "unix://"+p.SocketPath, platforms, true, true, nil, "", nil); err != nil {
				return err
			}
		}

		if use {
			ep, err := dockerutil.GetCurrentEndpoint(dockerCli)
			if err != nil {
				return err
			}

			if err := txn.SetCurrent(ep, name, false, false); err != nil {
				return err
			}
		}

		if err := txn.Save(ng); err != nil {
			return err
		}

		return nil
	})
}

func newCleanupBuildxCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Unregisters Namespace Remote builders from buildx.",
	}

	name := cmd.Flags().String("name", defaultBuilder, "The name of the builder we setup.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		dockerCli, err := command.NewDockerCli()
		if err != nil {
			return err
		}

		if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
			return err
		}

		return withStore(dockerCli, func(txn *store.Txn) error {
			return txn.Remove(*name)
		})
	})

	return cmd
}

func newWireBuildxCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wire",
		Short: "Wires a previously setup proxy setup.",
	}

	name := cmd.Flags().String("name", defaultBuilder, "The name of the builder we setup.")
	use := cmd.Flags().Bool("use", false, "If true, changes the current builder to nsc-remote.")
	stateDir := cmd.Flags().String("state", "", "Where the proxies live.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *stateDir == "" {
			return fnerrors.New("--state is required")
		}

		dockerCli, err := command.NewDockerCli()
		if err != nil {
			return err
		}

		if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
			return err
		}

		var md buildxMetadata
		if err := files.ReadJson(filepath.Join(*stateDir, "metadata.json"), &md); err != nil {
			return err
		}

		return wireBuildx(dockerCli, *name, *use, md)
	})

	return cmd
}

func determineAvailable(ctx context.Context) ([]api.BuildPlatform, error) {
	profile, err := api.GetProfile(ctx, api.Endpoint)
	if err != nil {
		return nil, err
	}

	avail := make([]api.BuildPlatform, len(profile.ClusterPlatform))
	for k, x := range profile.ClusterPlatform {
		avail[k] = api.BuildPlatform(x)
	}

	return avail, nil
}

func banner(ctx context.Context, name string, use bool, native []api.BuildPlatform) string {
	w := wordwrap.NewWriter(80)
	style := colors.Ctx(ctx)

	fmt.Fprint(w, style.Highlight.Apply("docker buildx"), " has been configured to use ",
		style.Highlight.Apply("Namespace Remote builders"), ".\n")

	fmt.Fprintln(w)
	fmt.Fprint(w, "Native building: ", strings.Join(bold(style, native), " and "), ".\n")

	if !use {
		fmt.Fprint(w, "\nThe default buildx builder was not changed, you can re-run with ", style.Highlight.Apply("--use"), " or run:\n")
		fmt.Fprintf(w, "\n  docker buildx use %s\n", name)
	}

	fmt.Fprintf(w, "\nStart a new terminal, and start building:\n")
	fmt.Fprintf(w, "\n  docker buildx build ...\n")

	fmt.Fprintln(w)
	fmt.Fprintln(w, style.Comment.Apply("Exiting will remove the configuration."))

	_ = w.Close()

	return strings.TrimSpace(w.String())
}

func bold[X any](style colors.Style, values []X) []string {
	result := make([]string, len(values))
	for k, x := range values {
		result[k] = style.Highlight.Apply(fmt.Sprintf("%v", x))
	}
	return result
}

func withStore(dockerCli *command.DockerCli, f func(*store.Txn) error) error {
	txn, release, err := storeutil.GetStore(dockerCli)
	if err != nil {
		return err
	}
	defer release()

	return f(txn)
}

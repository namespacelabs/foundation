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

const (
	metadataFile      = "metadata.json"
	defaultBuilder    = "nsc-remote"
	proxyDir          = "proxy"
	buildkitProxyPath = "buildkit/" + proxyDir
)

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

		state, err := ensureStateDir(*stateDir, buildkitProxyPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(console.Debug(ctx), "Using state path %q\n", state)

		if proxyAlreadyExists(state) {
			console.SetStickyContent(ctx, "build", existingProxyMessage(*stateDir))
			return nil
		}

		md := buildxMetadata{
			NodeGroupName: *name,
		}
		for _, p := range available {
			sockPath := filepath.Join(state, fmt.Sprintf("%s.sock", p))
			md.Instances = append(md.Instances, buildxInstanceMetadata{
				Platform:   p,
				SocketPath: sockPath,
			})
		}

		var instances []*BuildClusterInstance
		for i, p := range md.Instances {
			// Always create one, in case it's needed below. This instance has a zero-ish cost if we never call NewConn.
			instance := NewBuildClusterInstance0(p.Platform)
			instances = append(instances, instance)

			if *background {
				if pid, err := startBackgroundProxy(ctx, p, *createAtStartup); err != nil {
					return err
				} else {
					md.Instances[i].Pid = pid
				}
			} else {
				md.Instances[i].Pid = os.Getpid()
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

		if err := files.WriteJson(filepath.Join(state, metadataFile), md, 0644); err != nil {
			return err
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

		// Print info message even if proxy goes in background
		console.SetStickyContent(ctx, "build", banner(ctx, *name, *use, available, *background))

		if *background {
			return nil
		}

		if err := eg.Wait(); err != nil {
			return err
		}

		if err := withStore(dockerCli, func(txn *store.Txn) error {
			return txn.Remove(*name)
		}); err != nil {
			return err
		}

		if err := os.RemoveAll(state); err != nil {
			return err
		}

		return nil
	})

	return cmd
}

func proxyAlreadyExists(stateDir string) bool {
	_, err := os.Stat(filepath.Join(stateDir, metadataFile))
	return !os.IsNotExist(err)
}

func existingProxyMessage(customStateDir string) string {
	if customStateDir != "" {
		return fmt.Sprintf(`Previous Buildx proxy configuration found in %s.
If you want to create a new proxy configuration, cleanup the older one first with:

   nsc docker buildx cleanup --state %s
`, customStateDir, customStateDir)
	} else {
		return `Previous Buildx proxy configuration found.
If you want to create a new proxy configuration, cleanup the older one first with:

   nsc docker buildx cleanup
`
	}
}

func ensureStateDir(specified, dir string) (string, error) {
	if specified == "" {
		return dirs.Ensure(dirs.Subdir(dir))
	}

	s, err := filepath.Abs(specified)
	if err != nil {
		return "", err
	}

	return dirs.Ensure(filepath.Join(s, proxyDir), nil)
}

type buildxMetadata struct {
	NodeGroupName string                   `json:"node_group_name"`
	Instances     []buildxInstanceMetadata `json:"instances"`
}

type buildxInstanceMetadata struct {
	Platform   api.BuildPlatform `json:"build_platform"`
	SocketPath string            `json:"socket_path"`
	Pid        int               `json:"pid"`
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

	stateDir := cmd.Flags().String("state", "", "If set, stores the proxy sockets in this directory.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		dockerCli, err := command.NewDockerCli()
		if err != nil {
			return err
		}

		if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
			return err
		}

		return withStore(dockerCli, func(txn *store.Txn) error {
			state, err := ensureStateDir(*stateDir, buildkitProxyPath)
			if err != nil {
				return err
			}

			if !proxyAlreadyExists(state) {
				console.SetStickyContent(ctx, "build", "State file not found. Nothing to cleanup.")
				return nil
			}

			var md buildxMetadata
			if err := files.ReadJson(filepath.Join(state, metadataFile), &md); err != nil {
				return err
			}

			for _, inst := range md.Instances {
				if inst.Pid > 0 {
					process, err := os.FindProcess(inst.Pid)
					if err != nil {
						return err
					}

					if err := process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
						return err
					}
				}
			}

			if err := os.RemoveAll(state); err != nil {
				console.SetStickyContent(ctx, "build",
					fmt.Sprintf("Warning: deleting state files in %s failed: %v", state, err))
			}

			if md.NodeGroupName != "" {
				return txn.Remove(md.NodeGroupName)
			}

			return nil
		})
	})

	return cmd
}

func newWireBuildxCommand(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "wire",
		Short:  "Wires a previously setup proxy setup.",
		Hidden: hidden,
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
		if err := files.ReadJson(filepath.Join(*stateDir, metadataFile), &md); err != nil {
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

func banner(ctx context.Context, name string, use bool, native []api.BuildPlatform, background bool) string {
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

	if !background {
		fmt.Fprintf(w, "\nStart a new terminal, and start building:\n")
		fmt.Fprintf(w, "\n  docker buildx build ...\n")

		fmt.Fprintln(w)
		fmt.Fprintln(w, style.Comment.Apply("Exiting will remove the configuration."))
	} else {
		fmt.Fprintf(w, "\nStart building:\n")
		fmt.Fprintf(w, "\n  docker buildx build ...\n")

		fmt.Fprintf(w, "\nYour remote builder context is running in the background. You can always clean it up with:\n")
		fmt.Fprintf(w, "\n  nsc docker buildx cleanup \n")
	}

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

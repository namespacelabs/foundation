// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/pkg/system"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progresswriter"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/framework/kubernetes/kubenaming"
	"namespacelabs.dev/foundation/internal/build/buildkit/bkkeychain"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	patchFile = "/tmp/namespace/changes.patch"
)

func NewLargeBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "large-build",
		Short:  "Stashes local changes and runs the specified command remotely.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	image := cmd.Flags().String("toolchain", "", "Base image containing all tools to run.")
	outFrom := cmd.Flags().String("output-from", "/out", "Which directory to capture for the final output.")
	outTo := cmd.Flags().String("output-to", "./out", "Where to download the final output.")
	plat := cmd.Flags().String("platform", "linux/amd64", "Set target platform for build.")
	commands := cmd.Flags().StringSlice("command", nil, "The commands to run.")
	cacheDirs := cmd.Flags().StringSlice("cache_dir", nil, "Which directories to cache.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *image == "" {
			return fnerrors.New("--image is required")
		}

		platformSpec, err := platform.ParsePlatform(*plat)
		if err != nil {
			return err
		}

		out, err := makeProgram(ctx, platformSpec, *image, *outFrom, *cacheDirs, *commands...)
		if err != nil {
			return err
		}

		def, err := out.Marshal(ctx)
		if err != nil {
			return err
		}

		bp, err := NewBuildClusterInstance(ctx, *plat)
		if err != nil {
			return err
		}

		sink := tasks.SinkFrom(ctx)
		cli, err := client.New(ctx, "buildkitd", client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return bp.NewConn(tasks.WithSink(ctx, sink))
		}))
		if err != nil {
			return err
		}

		done := console.EnterInputMode(ctx)
		defer done()

		// not using shared context to not disrupt display but let is finish reporting errors
		pw, err := progresswriter.NewPrinter(context.Background(), os.Stderr, "auto")
		if err != nil {
			return err
		}

		eg := executor.New(ctx, "largebuild")

		eg.Go(func(ctx context.Context) error {
			var attachable []session.Attachable

			dockerConfig := config.LoadDefaultConfigFile(console.Stderr(ctx))

			attachable = append(attachable, bkkeychain.Wrapper{
				Context:     ctx,
				ErrorLogger: io.Discard,
				Keychain:    keychain{},
				Fallback:    authprovider.NewDockerAuthProvider(dockerConfig).(auth.AuthServer),
			})

			if _, err := cli.Solve(ctx, def, client.SolveOpt{
				Exports: []client.ExportEntry{
					{
						Type:      client.ExporterLocal,
						OutputDir: *outTo,
					},
				},
				Session: attachable,
			}, pw.Status()); err != nil {
				return err
			}

			return nil
		})

		eg.Go(func(_ context.Context) error {
			<-pw.Done()
			return pw.Err()
		})

		return eg.Wait()
	})

	return cmd
}

func makeProgram(ctx context.Context, platform specs.Platform, baseImage, outFrom string, cacheDirs []string, commands ...string) (llb.State, error) {
	var zero llb.State

	// E.g. github.com/username/reponame
	remote, err := git.RemoteUrl(ctx, ".")
	if err != nil {
		return zero, err
	}

	branch, err := git.CurrentBranch(ctx, ".")
	if err != nil {
		return zero, err
	}

	diff, _, err := git.RunGit(ctx, ".", "diff", "HEAD")
	if err != nil {
		return zero, err
	}

	var stashBytes []byte
	if len(diff) > 0 {
		// create a stash
		if _, _, err := git.RunGit(ctx, ".", "stash", "save", "-u"); err != nil {
			return zero, err
		}

		// grab stash content
		stashBytes, _, err = git.RunGit(ctx, ".", "stash", "show", "-p", "-u")
		if err != nil {
			return zero, err
		}

		// restore local state
		if _, _, err = git.RunGit(ctx, ".", "stash", "pop"); err != nil {
			return zero, err
		}

	}

	source := llb.Git(remote, branch)

	if len(stashBytes) > 0 {
		stash := llbutil.AddFile(llb.Scratch(), patchFile, 0700, stashBytes)
		applyStash := llbutil.Image(baseImage, platform).
			Dir("/source").
			Run(llb.Shlexf("git apply %s", filepath.Join("/stash", patchFile)))

		applyStash.AddMount("/stash", stash)
		source = applyStash.AddMount("/source", source)
	}

	base := llbutil.Image(baseImage, platform).
		Dir("/source").
		AddEnv("PATH", "/usr/local/go/bin:"+system.DefaultPathEnv("linux"))

	out := llb.Scratch()

	for _, cmd := range commands {
		run := base.Run(llb.Shlex(cmd))

		for _, d := range cacheDirs {
			run.AddMount(d, llb.Scratch(), llb.AsPersistentCacheDir(normalizeName(d), llb.CacheMountShared))
		}

		source = run.AddMount("/source", source)
		out = run.AddMount(outFrom, out)
		base = run.Root()
	}

	return out, nil
}

func normalizeName(str string) string {
	return kubenaming.DomainFragLike(str)
}

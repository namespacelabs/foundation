// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"net"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	workImage = "golang:1.21.1"
	patchFile = "/tmp/namespace/changes.patch"
)

func NewLargeBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "large-build",
		Short:  "Stashes local changes and runs the specified command remotely.",
		Args:   cobra.ArbitraryArgs,
		Hidden: true,
	}

	image := cmd.Flags().String("image", "", "Base image containing all tools to run.")
	outFrom := cmd.Flags().String("output-from", "/out", "Which directory to capture for the final output.")
	outTo := cmd.Flags().String("output-to", "./out", "Where to download the final output.")
	plat := cmd.Flags().String("platform", "linux/amd64", "Set target platform for build.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *image == "" {
			return fnerrors.New("--image is required")
		}

		platformSpec, err := platform.ParsePlatform(*plat)
		if err != nil {
			return err
		}

		// E.g. github.com/username/reponame
		remote, err := git.RemoteUrl(ctx, ".")
		if err != nil {
			return err
		}

		branch, err := git.CurrentBranch(ctx, ".")
		if err != nil {
			return err
		}

		diff, _, err := git.RunGit(ctx, ".", "diff", "HEAD")
		if err != nil {
			return err
		}

		var stashBytes []byte
		if len(diff) > 0 {
			// create a stash
			if _, _, err := git.RunGit(ctx, ".", "stash", "save", "-u"); err != nil {
				return err
			}

			// grab stash content
			stashBytes, _, err = git.RunGit(ctx, ".", "stash", "show", "-p", "-u")
			if err != nil {
				return err
			}

			// restore local state
			if _, _, err = git.RunGit(ctx, ".", "stash", "pop"); err != nil {
				return err
			}

		}

		source := llb.Git(remote, branch)

		if len(stashBytes) > 0 {
			stash := llbutil.AddFile(llb.Scratch(), patchFile, 0700, stashBytes)
			applyStash := llbutil.Image(workImage, platformSpec).
				Dir("/source").
				Run(llb.Shlexf("git apply %s", filepath.Join("/stash", patchFile)))

			applyStash.AddMount("/stash", stash)
			source = applyStash.AddMount("/source", source)
		}

		toolchain := llbutil.Image(*image, platformSpec).
			Dir("/source").
			Run(llb.Shlex(strings.Join(args, " ")))
		toolchain.AddMount("/source", source)
		out := toolchain.AddMount(*outFrom, llb.Scratch())

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

		if _, err := cli.Solve(ctx, def, client.SolveOpt{
			Exports: []client.ExportEntry{{
				Type:      client.ExporterLocal,
				OutputDir: *outTo,
			}},
		}, nil); err != nil {
			return err
		}

		return nil
	})

	return cmd
}

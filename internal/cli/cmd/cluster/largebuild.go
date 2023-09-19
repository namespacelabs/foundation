// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"net"
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
	patchFile = "/tmp/namespace/changes.patch"
	workDir   = "/work"
)

func NewLargeBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "large-build",
		Short:  "Stashes local changes and runs the specified command remotely.",
		Args:   cobra.ArbitraryArgs,
		Hidden: true,
	}

	image := cmd.Flags().String("image", "", "Base image containing all tools to run.")
	plat := cmd.Flags().String("platform", "linux/amd64", "Set target platform for build.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *image == "" {
			return fnerrors.New("--image is required")
		}

		platformSpec, err := platform.ParsePlatform(*plat)
		if err != nil {
			return err
		}

		base := llbutil.Image(*image, platformSpec)

		// E.g. github.com/username/reponame
		remote, err := git.RemoteUrl(ctx, ".")
		if err != nil {
			return err
		}

		branch, err := git.CurrentBranch(ctx, ".")
		if err != nil {
			return err
		}

		base = base.
			Run(llb.Shlexf("git clone -b %s https://%s %s", branch, remote, workDir)).Root()

		// create a stash
		if _, _, err := git.RunGit(ctx, ".", "stash", "save", "-u"); err != nil {
			return err
		}

		// grab stash content
		stash, _, err := git.RunGit(ctx, ".", "stash", "show", "-p", "-u")
		if err != nil {
			return err
		}

		// restore local state
		if _, _, err = git.RunGit(ctx, ".", "stash", "pop"); err != nil {
			return err
		}

		base = llbutil.AddFile(base, patchFile, 0700, stash).
			Dir(workDir).
			Run(llb.Shlexf("git apply %s", patchFile)).
			Run(llb.Shlex(strings.Join(args, " "))).Root()

		def, err := base.Marshal(ctx)
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

		if _, err := cli.Solve(ctx, def, client.SolveOpt{}, nil); err != nil {
			return err
		}

		return nil
	})

	return cmd
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nsbuild

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/utils/pointer"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/integrations/golang"
	"namespacelabs.dev/foundation/internal/parsing"
	gosdk "namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewNsBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "ns-build",
		Short:  "ns build related entries.",
		Hidden: true,
	}

	cmd.AddCommand(newInstall())
	cmd.AddCommand(newInstallDev())

	return cmd
}

func newInstall() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Run `go install ns`.",
	}

	env := fncobra.EnvFromValue(cmd, pointer.String("dev"))

	return fncobra.With(cmd, func(ctx context.Context) error {
		return run(ctx, *env, "install", "./cmd/ns")
	})
}

func newInstallDev() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-dev",
		Short: "Run `go install nsdev`.",
	}

	env := fncobra.EnvFromValue(cmd, pointer.String("dev"))

	return fncobra.With(cmd, func(ctx context.Context) error {
		return run(ctx, *env, "install", "./cmd/nsdev")
	})
}

func run(ctx context.Context, env cfg.Context, what, cmdpkg string) error {
	pl := parsing.NewPackageLoader(env)
	loc, err := pl.Resolve(ctx, "namespacelabs.dev/foundation")
	if err != nil {
		return err
	}

	bin, err := golang.FromLocation(loc, "cmd/nsdev")
	if err != nil {
		return err
	}

	matched, err := gosdk.MatchSDK(bin.GoVersion, host.HostPlatform())
	if err != nil {
		return err
	}

	sdk, err := compute.GetValue(ctx, matched)
	if err != nil {
		return err
	}

	return golang.RunGo(ctx, loc, sdk, what, "-v", cmdpkg)
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package version

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"

	"github.com/kr/text"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/cli/versioncheck"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema/storage"
)

func NewVersionCmd() *cobra.Command {
	var (
		buildInfo bool
		short     bool
	)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Outputs the compiled version of Namespace.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			if buildInfo {
				info, ok := debug.ReadBuildInfo()
				if !ok {
					return fnerrors.InternalError("buildinfo is missing")
				}
				fmt.Fprintln(console.Stdout(ctx), info.String())
				return nil
			}

			v, err := CollectVersionInfo()
			if err != nil {
				return err
			}

			if short {
				fmt.Fprintln(console.Stdout(ctx), v.Binary.GetVersion())
				return nil
			}

			out := console.Stdout(ctx)
			FormatVersionInfo(out, v)
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVar(&buildInfo, "build_info", buildInfo, "Output all of build info.")
	cmd.Flags().BoolVar(&short, "short", false, "Only print the version number.")
	cmd.MarkFlagsMutuallyExclusive("build_info", "short")

	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newEnsureCmd())
	cmd.AddCommand(newCheckCmd())

	return cmd
}

func newEnsureCmd() *cobra.Command {
	var atLeast string

	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Ensures the current binary is at least a given version, updating if needed.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			if atLeast == "" {
				return fnerrors.New("--at_least is required")
			}

			current := version.Tag
			if current == version.DevelopmentBuildVersion {
				fmt.Fprintf(console.Stdout(ctx), "Development build, skipping version check.\n")
				return nil
			}

			currentV := current
			if currentV != "" && currentV[0] != 'v' {
				currentV = "v" + currentV
			}

			requiredV := atLeast
			if requiredV != "" && requiredV[0] != 'v' {
				requiredV = "v" + requiredV
			}

			if semver.Compare(currentV, requiredV) >= 0 {
				fmt.Fprintf(console.Stdout(ctx), "Already up to date (version %s satisfies >= %s).\n", current, atLeast)
				return nil
			}

			fmt.Fprintf(console.Stdout(ctx), "Version %s is older than %s, updating...\n", current, atLeast)
			return nsboot.ForceUpdate(ctx, "nsc")
		}),
	}

	cmd.Flags().StringVar(&atLeast, "at_least", "", "Minimum required version (e.g. 0.0.481).")

	return cmd
}

func newCheckCmd() *cobra.Command {
	var (
		latest  bool
		atLeast string
		quiet   bool
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Checks whether the current binary is up to date.",
		Long: `Checks whether the current binary is up to date.

By default (--latest), checks if a newer version is available.
With --at_least, checks if the current version satisfies a minimum version constraint.

Exits with code 0 if the version is up to date or the constraint is met.
Exits with code 2 if the version is outdated or the constraint is not met.`,
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"ns.skip-version-check": "true",
		},

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			current := version.Tag
			if current == version.DevelopmentBuildVersion {
				fmt.Fprintf(console.Stdout(ctx), "Development build, skipping version check.\n")
				return nil
			}

			currentV := current
			if currentV != "" && currentV[0] != 'v' {
				currentV = "v" + currentV
			}

			if atLeast != "" {
				requiredV := atLeast
				if requiredV != "" && requiredV[0] != 'v' {
					requiredV = "v" + requiredV
				}

				if semver.Compare(currentV, requiredV) >= 0 {
					if !quiet {
						fmt.Fprintf(console.Stderr(ctx), "✔ Current version %s matches constraint >= %s.\n", current, atLeast)
					}
					return nil
				}

				fmt.Fprintf(console.Stderr(ctx), "✘ Version %s is older than required version %s.\n", current, atLeast)
				return fnerrors.ExitWithCode(fmt.Errorf("version check failed"), 2)
			}

			// --latest mode (default).
			ver, err := version.Current()
			if err != nil {
				return err
			}

			status, err := versioncheck.CheckRemote(ctx, ver, "nsc")
			if err != nil {
				return err
			}

			if status != nil && status.NewVersion {
				fmt.Fprintf(console.Stderr(ctx), "✘ A newer version is available: %s (current: %s).\n", status.Version, current)
				return fnerrors.ExitWithCode(fmt.Errorf("newer version available"), 2)
			}

			if !quiet {
				fmt.Fprintf(console.Stderr(ctx), "✔ Already on latest version.\n")
			}
			return nil
		}),
	}

	cmd.Flags().BoolVar(&latest, "latest", true, "Check if a newer version is available.")
	cmd.Flags().StringVar(&atLeast, "at_least", "", "Check if the current version is at least the given version (does not check for newer versions).")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress successful output, only prints if version is outdated.")
	cmd.MarkFlagsMutuallyExclusive("latest", "at_least")

	return cmd
}

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Updates the current binary to the latest version.",
		Aliases: []string{"update-ns"},
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			"ns.skip-version-check": "true",
		},

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return nsboot.ForceUpdate(ctx, "nsc")
		}),
	}

	return cmd
}

type VersionInfo struct {
	Binary         *storage.NamespaceBinaryVersion `json:"binary"`
	GOOS           string                          `json:"GOOS"`
	GOARCH         string                          `json:"GOARCH"`
	APIVersion     int                             `json:"api_version"`
	CacheVersion   int                             `json:"cache_version"`
	ToolAPIVersion int                             `json:"tool_api_version"`
}

func CollectVersionInfo() (*VersionInfo, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, fnerrors.InternalError("buildinfo is missing")
	}

	v, err := version.VersionFrom(info)
	if err != nil {
		return nil, err
	}

	if v.GitCommit == "" {
		return nil, fnerrors.InternalError("binary does not include version information")
	}

	return &VersionInfo{
		Binary:         v,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		APIVersion:     versions.Builtin().APIVersion,
		CacheVersion:   versions.Builtin().CacheVersion,
		ToolAPIVersion: versions.ToolAPIVersion,
	}, nil
}

func FormatVersionInfo(out io.Writer, v *VersionInfo) {
	FormatBinaryVersion(out, v.Binary)
	x := text.NewIndentWriter(out, []byte("  ")) // align with FormatBinaryVersion
	fmt.Fprintf(x, "architecture %s/%s\n", v.GOOS, v.GOARCH)
	fmt.Fprintf(x, "internal api %d (cache=%d tools=%d)\n", v.APIVersion, v.CacheVersion, v.ToolAPIVersion)
}

func FormatBinaryVersion(out io.Writer, v *storage.NamespaceBinaryVersion) {
	fmt.Fprintf(out, "version %s (commit %s)\n", v.Version, v.GitCommit)

	x := text.NewIndentWriter(out, []byte("  "))

	if v.BuildTimeStr != "" {
		fmt.Fprintf(x, "commit date %s\n", v.BuildTimeStr)
	}
	if v.Modified {
		fmt.Fprintf(x, "built from modified repo\n")
	}
}

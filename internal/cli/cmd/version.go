// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"

	"github.com/kr/text"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema/storage"
)

func NewVersionCmd() *cobra.Command {
	var buildInfo bool

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

			out := console.Stdout(ctx)
			FormatVersionInfo(out, v)
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVar(&buildInfo, "build_info", buildInfo, "Output all of build info.")

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

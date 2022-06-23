// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/kr/text"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
)

func NewVersionCmd() *cobra.Command {
	var buildInfo bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Outputs the compiled version of Foundation.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			info, ok := debug.ReadBuildInfo()
			if !ok {
				return fnerrors.InternalError("buildinfo is missing")
			}

			if buildInfo {
				fmt.Fprintln(console.Stdout(ctx), info.String())
				return nil
			}

			v, err := version.VersionFrom(info)
			if err != nil {
				return err
			}

			if v.GitCommit == "" {
				return fnerrors.InternalError("binary does not include version information")
			}

			out := console.Stdout(ctx)
			fmt.Fprintf(out, "ns version %s (commit %s)\n", v.Version, v.GitCommit)

			x := text.NewIndentWriter(out, []byte("  "))

			if v.BuildTimeStr != "" {
				fmt.Fprintf(x, "commit date %s\n", v.BuildTimeStr)
			}
			if v.Modified {
				fmt.Fprintf(x, "built from modified repo\n")
			}

			fmt.Fprintf(x, "architecture %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Fprintf(x, "internal api %d (cache=%d tools=%d)\n", versions.APIVersion, versions.CacheVersion, versions.ToolAPIVersion)

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVar(&buildInfo, "build_info", buildInfo, "Output all of build info.")

	return cmd
}

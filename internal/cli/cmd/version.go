// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
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

			var modified bool
			var revision, time string

			for _, n := range info.Settings {
				switch n.Key {
				case "vcs.revision":
					revision = n.Value
				case "vcs.time":
					time = n.Value
				case "vcs.modified":
					modified = n.Value == "true"
				}
			}

			if revision == "" {
				return fnerrors.InternalError("binary does not include version information")
			}

			out := console.Stdout(ctx)
			fmt.Fprintf(out, "fn version %s", revision)

			hints := []string{}
			if time != "" {
				hints = append(hints, fmt.Sprintf("built at %s", time))
			}
			if modified {
				hints = append(hints, "from a modified repo")
			}
			hints = append(hints, fmt.Sprintf("on %s/%s", runtime.GOOS, runtime.GOARCH))

			if len(hints) > 0 {
				fmt.Fprintf(out, " (%s)\n", strings.Join(hints, " "))
			}
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVar(&buildInfo, "build_info", buildInfo, "Output all of build info.")

	return cmd
}
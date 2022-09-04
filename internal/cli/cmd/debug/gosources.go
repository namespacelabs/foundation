// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/languages/golang"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/std/planning"
)

func newGoSourcesCmd() *cobra.Command {
	var (
		env     planning.Context
		locs    fncobra.Locations
		servers fncobra.Servers
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "go-sources",
			Short: "List go sources of a package."}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, &fncobra.ParseLocationsOpts{RequireSingle: true}),
			fncobra.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			t := servers.Servers[0]

			platforms, err := runtime.TargetPlatforms(ctx, env)
			if err != nil {
				return err
			}

			res, err := golang.ComputeSources(ctx, t.Module().Abs(), t, build.PlatformsOrOverrides(platforms))
			if err != nil {
				return err
			}

			out := console.Stdout(ctx)

			for _, dep := range res.Deps {
				fmt.Fprintf(out, "dep: %s\n", dep)
			}

			for d, to := range res.DepTo {
				fmt.Fprintf(out, "%s --> %s\n", d, strings.Join(to, ", "))
			}

			for d, to := range res.GoFiles {
				fmt.Fprintf(out, "files: %s --> %s\n", d, strings.Join(to, ", "))
			}

			return nil
		})
}

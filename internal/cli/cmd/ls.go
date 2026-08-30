// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/kr/text"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewLsCmd() *cobra.Command {
	var (
		env     cfg.Context
		locs    fncobra.Locations
		details bool
		kind    string
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:     "ls [path/to/package | module/path/...]",
			Short:   "List packages or objects defined in a package.",
			Aliases: []string{"list"},
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVarP(&details, "details", "l", false, "If set to true, prints the contents of the listed packages.")
			flags.StringVarP(&kind, "kind", "k", "", "If set, filters servers, services or tests.")
		}).
		With(
			fncobra.HardcodeEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true})).
		Do(func(ctx context.Context) error {
			style := colors.Ctx(ctx)
			stdout := console.Stdout(ctx)
			pl := parsing.NewPackageLoader(env)

			if !slices.Contains([]string{"", "server", "servers", "service", "services", "test", "tests", "binary", "binaries"}, kind) {
				return fnerrors.Newf("bad kind %q", kind)
			}

			for _, l := range locs.Locations {
				if kind == "" {
					fmt.Fprintf(stdout, "%s\n", l)
				}

				if locs.UserSpecified || details || kind != "" {
					pkg, err := pl.LoadByName(ctx, l.AsPackageName())
					if err != nil {
						return err
					}

					if kind != "" {
						switch kind {
						case "server", "servers":
							if r := pkg.Server; r != nil {
								fmt.Fprintf(stdout, "%s\n", r.PackageName)
							}

						case "service", "services":
							if r := pkg.Service; r != nil {
								fmt.Fprintf(stdout, "%s\n", r.PackageName)
							}

						case "test", "tests":
							for _, r := range pkg.Tests {
								fmt.Fprintf(stdout, "%s:%s\n", r.PackageName, r.Name)
							}

						case "binary", "binaries":
							for _, r := range pkg.Binaries {
								fmt.Fprintf(stdout, "%s:%s\n", r.PackageName, r.Name)
							}
						}

						continue
					}

					resout := text.NewIndentWriter(stdout, []byte("    "))
					if pkg.Extension != nil {
						fmt.Fprintf(resout, "%s\n", style.Comment.Apply("Extension"))
					}
					if pkg.Service != nil {
						fmt.Fprintf(resout, "%s\n", style.Comment.Apply("Service"))
					}
					if pkg.Server != nil {
						fmt.Fprintf(resout, "%s\n", style.Comment.Apply("Server"))
					}
					for _, r := range pkg.Tests {
						fmt.Fprintf(resout, "%s :%s\n",
							style.Comment.Apply("Test"),
							r.Name)
					}
					for _, r := range pkg.Binaries {
						fmt.Fprintf(resout, "%s :%s\n",
							style.Comment.Apply("Binary"),
							r.Name)
					}
					for _, r := range pkg.Secrets {
						fmt.Fprintf(resout, "%s :%s %s\n",
							style.Comment.Apply("Secret"),
							r.Name,
							style.Comment.Apply(fmt.Sprintf("(%s)", r.Description)))
					}
					for _, r := range pkg.Volumes {
						fmt.Fprintf(resout, "%s :%s (%s) <- %s\n",
							style.Comment.Apply("Volume"),
							r.Name,
							r.Kind,
							r.Owner)
					}
					for _, r := range pkg.Resources {
						fmt.Fprintf(resout, "%s :%s (%s)\n",
							style.Comment.Apply("Resource"),
							r.ResourceRef.Name,
							formatPkgRef(style, r.Spec.Class.Ref))
						fmt.Fprintf(resout, "      %s %s\n",
							style.Comment.Apply("<-"),
							r.Spec.Provider.Spec.PackageName)
					}
					for _, r := range pkg.ResourceClasses {
						fmt.Fprintf(resout, "%s :%s\n",
							style.Comment.Apply("ResourceClass"),
							r.Ref.Name)
					}
					for _, r := range pkg.ResourceProviders {
						fmt.Fprintf(resout, "%s %s\n",
							style.Comment.Apply("ResourceProvider"),
							formatPkgRef(style, r.Spec.ProvidesClass))
					}
					fmt.Fprintln(resout)
				}
			}

			return nil
		})
}

func formatPkgRef(style colors.Style, pr *schema.PackageRef) string {
	return fmt.Sprintf("%s:%s", style.LessRelevant.Apply(pr.PackageName), pr.Name)
}

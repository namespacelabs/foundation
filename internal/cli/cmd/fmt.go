// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewFmtCmd() *cobra.Command {
	var (
		env planning.Context
	)

	all := false
	check := false

	return fncobra.Cmd(&cobra.Command{
		Use:   "fmt",
		Short: "Format foundation configurations of all packages in the workspace.",
		Args:  cobra.NoArgs,
	}).
		With(fncobra.HardcodeEnv(&env, "dev")).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVar(&all, "all", all, "If set to true, walks through all directories to look for .cue files to format, instead of all packages.")
			flags.BoolVar(&check, "check", check, "If set to true, fails if a file would have to be updated.")
		}).
		Do(func(ctx context.Context) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			opts := fnfs.WriteFileExtendedOpts{
				AnnounceWrite: console.Stdout(ctx),
				FailOverwrite: check,
			}

			var errs []error
			if !all {
				if err := walkSchemas(ctx, env, root, func(loc fnfs.Location, name string) {
					if err := fncue.Format(ctx, root.ReadWriteFS(), loc, name, opts); err != nil {
						errs = append(errs, err)
					}
				}); err != nil {
					return err
				}
			} else {
				if err := filepath.WalkDir(root.Abs(), func(path string, de fs.DirEntry, err error) error {
					if err != nil {
						return err
					}

					switch {
					case de.IsDir():
						return nil

					case filepath.Ext(path) == ".cue":
						rel, err := filepath.Rel(root.Abs(), filepath.Dir(path))
						if err != nil {
							return err
						}

						if err := fncue.Format(ctx, root.ReadWriteFS(), root.RelPackage(rel), filepath.Base(path), opts); err != nil {
							errs = append(errs, err)
						}
					}

					return nil
				}); err != nil {
					return err
				}
			}

			return multierr.New(errs...)
		})
}

func walkSchemas(ctx context.Context, env planning.Context, root *workspace.Root, f func(fnfs.Location, string)) error {
	list, err := workspace.ListSchemas(ctx, env, root)
	if err != nil {
		return err
	}

	for _, e := range list.Locations {
		ents, err := os.ReadDir(filepath.Join(root.Abs(), e.RelPath))
		if err != nil {
			fmt.Fprintln(console.Stderr(ctx), "failed to readdir", err)
			continue
		}

		for _, ent := range ents {
			if ent.IsDir() || filepath.Ext(ent.Name()) != ".cue" {
				continue
			}

			f(e, ent.Name())
		}
	}

	return nil
}

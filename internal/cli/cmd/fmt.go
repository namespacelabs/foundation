// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/karrick/godirwalk"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewFmtCmd() *cobra.Command {
	all := false

	cmd := &cobra.Command{
		Use:   "fmt",
		Short: "Format foundation configurations of the specified packages.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			if !all {
				return walkSchemas(ctx, root, func(loc fnfs.Location, name string) {
					fncue.Format(console.Stdout(ctx), root, loc, name)
				})
			} else {
				return godirwalk.Walk(root.Abs(), &godirwalk.Options{
					Callback: func(path string, directoryEntry *godirwalk.Dirent) error {
						if directoryEntry.IsDir() || filepath.Ext(path) != ".cue" {
							return nil
						}

						rel, err := filepath.Rel(root.Abs(), filepath.Dir(path))
						if err != nil {
							return err
						}

						fncue.Format(console.Stdout(ctx), root, root.RelPackage(rel), filepath.Base(path))
						return nil
					},
				})
			}
		}),
	}

	cmd.Flags().BoolVar(&all, "all", all, "If set to true, walks through all directories to look for .cue files to format, instead of all packages.")

	return cmd
}

func walkSchemas(ctx context.Context, root *workspace.Root, f func(fnfs.Location, string)) error {
	list, err := workspace.ListSchemas(ctx, root)
	if err != nil {
		return err
	}

	for _, e := range list.Locations {
		ents, err := ioutil.ReadDir(filepath.Join(root.Abs(), e.RelPath))
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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/inputs"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cue"
	"namespacelabs.dev/foundation/workspace/source/codegen"
)

func serverName(loc fnfs.Location) string {
	var name string
	base := filepath.Base(loc.RelPath)
	dir := filepath.Dir(loc.RelPath)
	if base != "server" {
		name = base
	} else if dir != "server" {
		name = dir
	}

	if name != "" && !strings.HasSuffix(name, "server") {
		return name + "server"
	}

	return name
}

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Creates a server.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := targetPackage(ctx, args, "service")
			if err != nil {
				return err
			}

			m := computeModel("server", serverName(loc))
			if !m.IsFinal() {
				// Form aborted
				return nil
			}

			opts := cue.GenServerOpts{Name: m.name.Value()}

			opts.Framework, err = inputs.SelectedFramework(m.framework)
			if err != nil {
				return err
			}

			if err := cue.GenerateServer(ctx, root.FS(), loc, opts); err != nil {
				return err
			}

			return codegen.ForLocations(ctx, root, []fnfs.Location{loc}, func(e codegen.GenerateError) {
				w := console.Stderr(ctx)
				fmt.Fprintf(w, "%s: %s failed:\n", e.PackageName, e.What)
				fnerrors.Format(w, true, e.Err)
			})
		}),
	}

	return cmd
}

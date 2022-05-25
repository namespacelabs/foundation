// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/inputs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cue"
	"namespacelabs.dev/foundation/workspace/source/codegen"
)

const serverSuffix = "server"

func serverName(loc fnfs.Location) string {
	var name string
	base := filepath.Base(loc.RelPath)
	dir := filepath.Dir(loc.RelPath)
	if base != serverSuffix {
		name = base
	} else if dir != serverSuffix {
		name = dir
	}

	if name != "" && !strings.HasSuffix(name, serverSuffix) {
		return name + serverSuffix
	}

	return name
}

func newServerCmd() *cobra.Command {
	use := "server"
	cmd := &cobra.Command{
		Use:   use,
		Short: "Creates a server.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := targetPackage(ctx, args, use)
			if err != nil {
				return err
			}

			m := computeModel(use, serverName(loc))
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

			// Aggregates and prints all accumulated codegen errors on return.
			errorCollector := fnerrors.ErrorCollector{}

			if err := codegen.ForLocationsGenCode(ctx, root, []fnfs.Location{loc}, errorCollector.Append); err != nil {
				return err
			}
			if !errorCollector.IsEmpty() {
				return errorCollector.Build()
			}
			return nil
		}),
	}

	return cmd
}

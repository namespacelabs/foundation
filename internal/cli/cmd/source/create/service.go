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
	"namespacelabs.dev/foundation/internal/frontend/proto"
	"namespacelabs.dev/foundation/workspace/source/codegen"
)

func serviceName(loc fnfs.Location) string {
	var name string
	base := filepath.Base(loc.RelPath)
	dir := filepath.Dir(loc.RelPath)
	if base != "service" {
		name = strcase.ToCamel(base)
	} else if dir != "service" {
		name = strcase.ToCamel(dir)
	}

	if name != "" && !strings.HasSuffix(strings.ToLower(name), "service") {
		return name + "Service"
	}

	return name
}

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Creates a service.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := targetPackage(ctx, args, "service")
			if err != nil {
				return err
			}

			m := computeModel("service", serviceName(loc))
			if !m.IsFinal() {
				// Form aborted
				return nil
			}

			fmwk, err := inputs.SelectedFramework(m.framework)
			if err != nil {
				return err
			}

			protoOpts := proto.GenServiceOpts{Name: m.name.Value(), Framework: fmwk}
			if err := proto.GenerateService(ctx, root.FS(), loc, protoOpts); err != nil {
				return err
			}

			cueOpts := cue.GenServiceOpts{Name: m.name.Value(), Framework: fmwk}
			if err := cue.GenerateService(ctx, root.FS(), loc, cueOpts); err != nil {
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

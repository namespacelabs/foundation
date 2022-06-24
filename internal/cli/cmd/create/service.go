// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cue"
	"namespacelabs.dev/foundation/internal/frontend/golang"
	"namespacelabs.dev/foundation/internal/frontend/proto"
	"namespacelabs.dev/foundation/schema"
)

const serviceSuffix = "service"

func newServiceCmd() *cobra.Command {
	use := "service"
	cmd := &cobra.Command{
		Use:   use,
		Short: "Creates a service.",
	}

	fmwkStr := frameworkFlag(cmd)
	name := cmd.Flags().String("name", "", "Service name.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		root, loc, err := targetPackage(ctx, args, use)
		if err != nil {
			return err
		}

		fmwk, err := selectFramework(ctx, "Which framework would you like to use?", fmwkStr)
		if err != nil {
			return err
		}

		if fmwk == nil {
			return context.Canceled
		}

		if *fmwk == schema.Framework_GO {
			if err := runGoInitCmdIfNeeded(ctx, root, cmd.Root()); err != nil {
				return err
			}
		}

		if *name == "" {
			*name, err = tui.Ask(ctx, "How would you like to name your service?",
				"A service's name should not contain private information, as it is used in various debugging references.\n\nIf a service exposes internet-facing handlers, then the service's name may also be part of public-facing endpoints.",
				serviceName(loc))
			if err != nil {
				return err
			}
		}

		if *name == "" {
			return context.Canceled
		}

		protoOpts := proto.GenServiceOpts{Name: *name, Framework: *fmwk}
		if err := proto.CreateProtoScaffold(ctx, root.FS(), loc, protoOpts); err != nil {
			return err
		}

		cueOpts := cue.GenServiceOpts{Name: *name, Framework: *fmwk}
		if err := cue.CreateServiceScaffold(ctx, root.FS(), loc, cueOpts); err != nil {
			return err
		}

		switch *fmwk {
		case schema.Framework_GO:
			goOpts := golang.GenServiceOpts{Name: *name}
			if err := golang.CreateGolangScaffold(ctx, root.FS(), loc, goOpts); err != nil {
				return err
			}
		}

		return codegenNode(ctx, root, loc)
	})

	return cmd
}

func serviceName(loc fnfs.Location) string {
	var name string
	base := filepath.Base(loc.RelPath)
	dir := filepath.Dir(loc.RelPath)
	if base != serviceSuffix {
		name = strcase.ToCamel(base)
	} else if dir != serviceSuffix {
		name = strcase.ToCamel(dir)
	}

	if name != "" && !strings.HasSuffix(strings.ToLower(name), serviceSuffix) {
		return name + strcase.ToCamel(serviceSuffix)
	}

	return name
}

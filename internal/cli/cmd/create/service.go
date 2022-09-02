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
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cue"
	"namespacelabs.dev/foundation/internal/frontend/golang"
	"namespacelabs.dev/foundation/internal/frontend/proto"
	"namespacelabs.dev/foundation/internal/frontend/web"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
)

const serviceSuffix = "service"

func newServiceCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
	var (
		targetPkg      targetPkg
		env            provision.Env
		fmwkFlag       string
		name           string
		httpBackendPkg string
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "service [path/to/package]",
			Short: "Creates a service.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&name, "name", "", "Service name.")
			flags.StringVar(&httpBackendPkg, "with_http_backend", "", "Package name of the API backend server.")
		}).
		With(parseTargetPkgWithDeps(&targetPkg, "service")...).
		With(
			fncobra.FixedEnv(&env, "dev"),
			withFramework(&fmwkFlag)).
		Do(func(ctx context.Context) error {

			fmwk, err := selectFramework(ctx, "Which framework would you like to use?", fmwkFlag)
			if err != nil {
				return err
			}

			if fmwk == nil {
				return context.Canceled
			}

			if *fmwk == schema.Framework_GO {
				if err := runGoInitCmdIfNeeded(ctx, targetPkg.Root, runCommand); err != nil {
					return err
				}
			}

			isNameUsed := *fmwk == schema.Framework_WEB

			if !isNameUsed {
				if name == "" {
					name, err = tui.Ask(ctx, "How would you like to name your service?",
						"A service's name should not contain private information, as it is used in various debugging references.\n\nIf a service exposes internet-facing handlers, then the service's name may also be part of public-facing endpoints.",
						serviceName(targetPkg.Loc))
					if err != nil {
						return err
					}
				}

				if name == "" {
					return context.Canceled
				}
			} else {
				name = ""
			}

			if *fmwk == schema.Framework_GO || *fmwk == schema.Framework_NODEJS {
				protoOpts := proto.GenServiceOpts{Name: name, Framework: *fmwk}
				if err := proto.CreateProtoScaffold(ctx, targetPkg.Root.FS(), targetPkg.Loc, protoOpts); err != nil {
					return err
				}
			}

			cueOpts := cue.GenServiceOpts{
				ExportedServiceName: name,
				Framework:           *fmwk,
				HttpBackendPkg:      httpBackendPkg,
			}
			if err := cue.CreateServiceScaffold(ctx, targetPkg.Root.FS(), targetPkg.Loc, cueOpts); err != nil {
				return err
			}

			switch *fmwk {
			case schema.Framework_GO:
				goOpts := golang.GenServiceOpts{Name: name}
				if err := golang.CreateServiceScaffold(ctx, targetPkg.Root.FS(), targetPkg.Loc, goOpts); err != nil {
					return err
				}
			case schema.Framework_WEB:
				webOpts := web.GenServiceOpts{}
				if err := web.CreateServiceScaffold(ctx, targetPkg.Root.FS(), targetPkg.Loc, webOpts); err != nil {
					return err
				}
			}

			return codegenNode(ctx, env, targetPkg.Root, targetPkg.Loc)
		})
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

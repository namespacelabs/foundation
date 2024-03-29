// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	"namespacelabs.dev/foundation/internal/frontend/scaffold"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

const serviceSuffix = "service"

func newServiceCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
	var (
		targetPkg      targetPkg
		env            cfg.Context
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
			fncobra.HardcodeEnv(&env, "dev"),
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

			if name == "" {
				name, err = tui.Ask(ctx, "How would you like to name your service?",
					"A service's name should not contain private information, as it is used in various debugging references.\n\nIf a service exposes internet-facing handlers, then the service's name may also be part of public-facing endpoints.",
					serviceName(targetPkg.Location))
				if err != nil {
					return err
				}
			}

			if name == "" {
				return context.Canceled
			}

			if *fmwk == schema.Framework_GO {
				protoOpts := scaffold.GenProtoServiceOpts{Name: name, Framework: *fmwk}
				if err := scaffold.CreateProtoScaffold(ctx, targetPkg.Root.ReadWriteFS(), targetPkg.Location, protoOpts); err != nil {
					return err
				}
			}

			cueOpts := scaffold.GenServiceOpts{
				ExportedServiceName: name,
				Framework:           *fmwk,
				HttpBackendPkg:      httpBackendPkg,
			}
			if err := scaffold.CreateServiceScaffold(ctx, targetPkg.Root.ReadWriteFS(), targetPkg.Location, cueOpts); err != nil {
				return err
			}

			switch *fmwk {
			case schema.Framework_GO:
				goOpts := scaffold.GenGoServiceOpts{Name: name}
				if err := scaffold.CreateGoServiceScaffold(ctx, targetPkg.Root.ReadWriteFS(), targetPkg.Location, goOpts); err != nil {
					return err
				}
			}

			return codegenNode(ctx, targetPkg.Root, env, targetPkg.Location)
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

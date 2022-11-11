// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package create

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/codegen/genpackage"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/scaffold"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

const serverSuffix = "server"

func newServerCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
	var (
		targetPkg targetPkg
		env       cfg.Context
		fmwkFlag  string
		name      string
	)
	grpcServices := []string{}
	httpServices := []string{}

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "server [path/to/package]",
			Short: "Creates a server.",
			Args:  cobra.RangeArgs(0, 1),
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&name, "name", "", "Server name.")
			flags.StringArrayVar(&grpcServices, "with_service", nil, "A service to wire to the server.")
			flags.StringArrayVar(&httpServices, "with_http_service", nil, "An HTTP service to wire to the server. Format: 'path:package'.")
		}).
		With(parseTargetPkgWithDeps(&targetPkg, "service")...).
		With(
			fncobra.HardcodeEnv(&env, "dev"),
			withFramework(&fmwkFlag)).
		Do(func(ctx context.Context) error {
			parsedHttpServices := []scaffold.HttpService{}
			for _, httpService := range httpServices {
				parts := strings.Split(httpService, ":")
				if len(parts) != 2 {
					return fnerrors.New("invalid http_services format: %s", httpService)
				}
				parsedHttpServices = append(parsedHttpServices, scaffold.HttpService{
					Path: parts[0],
					Pkg:  parts[1],
				})
			}

			fmwk, err := selectFramework(ctx, "Which framework are your services in?", fmwkFlag)
			if err != nil {
				return err
			}

			if fmwk == nil {
				return context.Canceled
			}

			var dependencies []string
			if *fmwk == schema.Framework_GO {
				if err := runGoInitCmdIfNeeded(ctx, targetPkg.Root, runCommand); err != nil {
					return err
				}

				dependencies = append(dependencies,
					"namespacelabs.dev/foundation/std/grpc/logging",
					"namespacelabs.dev/foundation/std/monitoring/prometheus",
					"namespacelabs.dev/foundation/std/monitoring/tracing/jaeger")
			}

			if name == "" {
				name, err = tui.Ask(ctx, "How would you like to name your server?",
					"A server's name is used to generate various production resource names and thus should not contain private information.",
					serverName(targetPkg.Location))
				if err != nil {
					return err
				}
			}

			if name == "" {
				return context.Canceled
			}

			opts := scaffold.GenServerOpts{Name: name, Framework: *fmwk, GrpcServices: grpcServices, Dependencies: dependencies, HttpServices: parsedHttpServices}
			if err := scaffold.CreateServerScaffold(ctx, targetPkg.Root.ReadWriteFS(), targetPkg.Location, opts); err != nil {
				return err
			}

			// Aggregates and prints all accumulated codegen errors on return.
			var errorCollector fnerrors.ErrorCollector

			if err := genpackage.ForLocationsGenCode(ctx, targetPkg.Root, env, []fnfs.Location{targetPkg.Location}, errorCollector.Append); err != nil {
				return err
			}

			return errorCollector.Error()
		})
}

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

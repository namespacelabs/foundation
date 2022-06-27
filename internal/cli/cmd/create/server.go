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
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/codegen"
)

const serverSuffix = "server"

func newServerCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
	use := "server"
	cmd := &cobra.Command{
		Use:   use,
		Short: "Creates a server.",
		Args:  cobra.RangeArgs(0, 1),
	}

	fmwkStr := frameworkFlag(cmd)
	name := cmd.Flags().String("name", "", "Server name.")
	grpcServices := cmd.Flags().StringArray("service", nil, "A service to wire to the server.")
	httpServices := cmd.Flags().StringArray("http_service", nil, "An HTTP service to wire to the server. Format: 'path:package'.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		parsedHttpServices := []cue.HttpService{}
		for _, httpService := range *httpServices {
			parts := strings.Split(httpService, ":")
			if len(parts) != 2 {
				return fnerrors.UserError(nil, "invalid http_services format: %s", httpService)
			}
			parsedHttpServices = append(parsedHttpServices, cue.HttpService{
				Path: parts[0],
				Pkg:  parts[1],
			})
		}

		root, loc, err := targetPackage(ctx, args, use)
		if err != nil {
			return err
		}

		fmwk, err := selectFramework(ctx, "Which framework are your services in?", fmwkStr)
		if err != nil {
			return err
		}

		if fmwk == nil {
			return context.Canceled
		}

		if *fmwk == schema.Framework_GO {
			if err := runGoInitCmdIfNeeded(ctx, root, runCommand); err != nil {
				return err
			}
		}

		if *name == "" {
			*name, err = tui.Ask(ctx, "How would you like to name your server?",
				"A server's name is used to generate various production resource names and thus should not contain private information.",
				serverName(loc))
			if err != nil {
				return err
			}
		}

		if *name == "" {
			return context.Canceled
		}

		opts := cue.GenServerOpts{Name: *name, Framework: *fmwk, GrpcServices: *grpcServices, HttpServices: parsedHttpServices}
		if err := cue.CreateServerScaffold(ctx, root.FS(), loc, opts); err != nil {
			return err
		}

		// Aggregates and prints all accumulated codegen errors on return.
		var errorCollector fnerrors.ErrorCollector

		if err := codegen.ForLocationsGenCode(ctx, root, []fnfs.Location{loc}, errorCollector.Append); err != nil {
			return err
		}

		return errorCollector.Error()
	})

	return cmd
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

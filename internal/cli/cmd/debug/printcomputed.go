// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/source/protos/resolver"
)

func newPrintComputedCmd() *cobra.Command {
	var (
		env        provision.Env
		locs       fncobra.Locations
		outputType string
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "print-computed",
			Short: "Load a service or server definition and print it's computed contents as JSON.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&outputType, "output", "json", "One of json, textproto.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, &fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			pl := workspace.NewPackageLoader(env)

			sealed, err := workspace.Seal(ctx, pl, locs.Locs[0].AsPackageName(), nil)
			if err != nil {
				return err
			}

			return output(ctx, pl, sealed.Proto, outputType)
		})
}

func output(ctx context.Context, pl workspace.Packages, msg proto.Message, outputType string) error {
	switch outputType {
	case "json":
		body, err := (protojson.MarshalOptions{
			UseProtoNames: true,
			Multiline:     true,
			Resolver:      resolver.NewResolver(ctx, pl),
		}).Marshal(msg)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "%s\n", body)

	case "textproto":
		out, err := prototext.MarshalOptions{
			Multiline: true,
			Resolver:  resolver.NewResolver(ctx, pl),
		}.Marshal(msg)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "%s\n", out)
	}

	return nil
}

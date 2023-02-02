// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	"namespacelabs.dev/foundation/internal/codegen/protos/resolver"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func newPrintComputedCmd() *cobra.Command {
	var (
		env        cfg.Context
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
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			pl := parsing.NewPackageLoader(env)

			sealed, err := parsing.Seal(ctx, pl, locs.Locations[0].AsPackageName(), nil)
			if err != nil {
				return err
			}

			return output(ctx, pl, &schema.Stack_Entry{Server: sealed.Result.Server, Node: sealed.Result.Nodes}, outputType)
		})
}

func output(ctx context.Context, pl pkggraph.PackageLoader, msg proto.Message, outputType string) error {
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

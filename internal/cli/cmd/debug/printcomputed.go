// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

func newPrintComputedCmd() *cobra.Command {
	var outputType string

	cmd := &cobra.Command{
		Use:   "print-computed",
		Short: "Load a service or server definition and print it's computed contents as JSON.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			pl := workspace.NewPackageLoader(root)

			sealed, err := workspace.Seal(ctx, pl, loc.AsPackageName(), nil)
			if err != nil {
				return err
			}

			return output(ctx, pl, sealed.Proto, outputType)
		}),
	}

	cmd.Flags().StringVar(&outputType, "output", "json", "One of json, textproto.")

	return cmd
}

func output(ctx context.Context, pl workspace.Packages, msg proto.Message, outputType string) error {
	switch outputType {
	case "json":
		body, err := (protojson.MarshalOptions{
			UseProtoNames: true,
			Multiline:     true,
			Resolver:      workspace.NewProviderProtoResolver(ctx, pl),
		}).Marshal(msg)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "%s\n", body)

	case "textproto":
		out, err := prototext.MarshalOptions{
			Multiline: true,
			Resolver:  workspace.NewProviderProtoResolver(ctx, pl),
		}.Marshal(msg)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "%s\n", out)
	}

	return nil
}
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/workspace/compute"
)

type CmdHandler func(context.Context, []string) error

type ArgParser interface {
	AddFlags(*cobra.Command)
	Parse(ctx context.Context, args []string) error
}

func RunE(f CmdHandler) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx, cancel := WithSigIntCancel(cmd.Context())
		defer cancel()

		return compute.Do(ctx, func(ctx context.Context) error {
			return f(ctx, args)
		})
	}
}

func CmdWithHandler(cmd *cobra.Command, f CmdHandler, argParsers ...ArgParser) *cobra.Command {
	for _, parser := range argParsers {
		parser.AddFlags(cmd)
	}

	cmd.RunE = RunE(func(ctx context.Context, args []string) error {
		for _, parser := range argParsers {
			if err := parser.Parse(ctx, args); err != nil {
				return err
			}
		}

		return f(ctx, args)
	})

	return cmd
}

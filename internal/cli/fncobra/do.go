// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

type CommandCtrl struct {
	cmd        *cobra.Command
	argParsers []ArgParser
}

func Cmd(cmd *cobra.Command) *CommandCtrl {
	return &CommandCtrl{
		cmd:        cmd,
		argParsers: []ArgParser{},
	}
}

func (c *CommandCtrl) With(argParser ...ArgParser) *CommandCtrl {
	c.argParsers = append(c.argParsers, argParser...)
	return c
}

func (c *CommandCtrl) WithFlags(f func(flags *pflag.FlagSet)) *CommandCtrl {
	return c.With(&simpleFlagParser{f})
}

func (c *CommandCtrl) Do(handler func(context.Context) error) *cobra.Command {
	return c.DoWithArgs(func(ctx context.Context, args []string) error {
		return handler(ctx)
	})
}

func (c *CommandCtrl) DoWithArgs(handler CmdHandler) *cobra.Command {
	for _, parser := range c.argParsers {
		parser.AddFlags(c.cmd)
	}

	c.cmd.RunE = RunE(func(ctx context.Context, args []string) error {
		for _, parser := range c.argParsers {
			if err := parser.Parse(ctx, args); err != nil {
				return err
			}
		}

		return handler(ctx, args)
	})

	return c.cmd
}

type simpleFlagParser struct {
	f func(flags *pflag.FlagSet)
}

func (p *simpleFlagParser) AddFlags(cmd *cobra.Command) {
	p.f(cmd.Flags())
}
func (p *simpleFlagParser) Parse(ctx context.Context, args []string) error { return nil }

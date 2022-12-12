// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
	"namespacelabs.dev/foundation/internal/cli/versioncheck"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/std/tasks"
)

type CmdHandler func(context.Context, []string) error

type ArgsParser interface {
	AddFlags(*cobra.Command)
	Parse(ctx context.Context, args []string) error
}

func RunE(handler CmdHandler) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return handler(cmd.Context(), args)
	}
}

func CheckVersion(ctx context.Context) func() {
	// Navigate with care: within the detach below we don't use the graph
	// context, but rather the incoming context. The reason for this is that
	// we need to have the ability to cancel the version check. THIS IS OK
	// BUT IT RELIES ON INTERNAL DETAILS! compute.Do invokes the callback in
	// the same executor where Detach() runs. So we're guaranteed to observe
	// cancelation here.
	ctx, cancel := context.WithCancel(ctx)

	compute.On(ctx).Detach(tasks.Action("ns.check-updated"), func(_ context.Context) error {
		status, err := versioncheck.CheckRemote(ctx, nil)
		if err != nil {
			fmt.Fprintf(console.Debug(ctx), "failed to check remote version: %v\n", err)
			return nil
		}

		if status == nil {
			return nil
		}

		if newVersion, ok := nsboot.CheckInvalidate(status); ok {
			compute.On(ctx).Cleanup(tasks.Action("ns.check-updated.notify").LogLevel(1), func(ctx context.Context) error {
				fmt.Fprintf(console.Stdout(ctx), "\n\n  A new version of ns is available (%s). It will be automatically downloaded on the next run.\n\n", newVersion)
				return nil
			})
		}

		return nil
	})

	return cancel
}

func RunInContext(ctx context.Context, handler func(context.Context) error, starter func(context.Context) func()) error {
	ctx, cancel := WithSigIntCancel(ctx)
	defer cancel()

	return compute.Do(ctx, func(ctx context.Context) error {
		if starter != nil {
			cancel := starter(ctx)
			if cancel != nil {
				defer cancel()
			}
		}
		return handler(ctx)
	})
}

type CommandCtrl struct {
	cmd        *cobra.Command
	argParsers []ArgsParser
}

func With(cmd *cobra.Command, handler func(context.Context) error) *cobra.Command {
	cmd.RunE = RunE(func(ctx context.Context, _ []string) error {
		return handler(ctx)
	})
	return cmd
}

func Cmd(cmd *cobra.Command) *CommandCtrl {
	return &CommandCtrl{
		cmd:        cmd,
		argParsers: []ArgsParser{},
	}
}

func (c *CommandCtrl) With(argParser ...ArgsParser) *CommandCtrl {
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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/cli/versioncheck"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/tasks"
)

const updateNotifyInterval = 24 * time.Hour

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

func DeferCheckVersion(ctx context.Context, command string) {
	ver, err := version.Current()
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to check current version: %v\n", err)
		return
	}

	if !version.ShouldCheckUpdate(ver) {
		return
	}

	if !shouldNotifyUpdate() {
		return
	}

	compute.On(ctx).BestEffort(tasks.Action(command+".check-updated"), func(ctx context.Context) error {
		status, err := versioncheck.CheckRemote(ctx, ver, command)
		if err != nil {
			fmt.Fprintf(console.Debug(ctx), "failed to check remote version: %v\n", err)
			return nil
		}

		if status == nil {
			return nil
		}

		if status.NewVersion {
			compute.On(ctx).Cleanup(tasks.Action(command+".check-updated.notify").LogLevel(1), func(ctx context.Context) error {
				_ = markUpdateNotified()
				fmt.Fprintf(console.Info(ctx), "\n\n  A new version of %s is available (%s).\n\n", command, status.Version)
				return nil
			})
		}

		return nil
	})
}

func updateNotifyPath() (string, error) {
	return dirs.ConfigSubdir("version-notify")
}

func shouldNotifyUpdate() bool {
	path, err := updateNotifyPath()
	if err != nil {
		return true
	}
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) >= updateNotifyInterval
}

func markUpdateNotified() error {
	path, err := updateNotifyPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	now := time.Now()
	if f, err := os.Create(path); err != nil {
		return err
	} else if err := f.Close(); err != nil {
		return err
	}
	return os.Chtimes(path, now, now)
}

func RunInContext(ctx context.Context, handler func(context.Context) error) error {
	ctx, cancel := WithSigIntCancel(ctx)
	defer cancel()

	return compute.Do(ctx, func(ctx context.Context) error {
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

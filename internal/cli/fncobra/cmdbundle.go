// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// CommandBundle tracks all actions stored while invoking the command in addition to
// diagnostic information about the environment and errors with serialized stack traces.
type CommandBundle struct {
	bundler *tasks.Bundler
	bundle  *tasks.Bundle
}

func NewCommandBundle() *CommandBundle {
	bundler := tasks.NewActionBundler()
	return &CommandBundle{
		bundler: bundler,
		bundle:  bundler.NewInMemoryBundle(),
	}
}

// RemoveStaleCommands removes all command bundles that are older than the configured bundle duration
// or if they exceed the configured number of bundles to keep.
func (c *CommandBundle) RemoveStaleCommands() error {
	return c.bundler.RemoveStaleBundles()
}

// RegisterCommand writes invocation information about the command to the bundle.
func (c *CommandBundle) RegisterCommand(cmd *cobra.Command, args []string) error {
	return c.bundle.WriteInvocationInfo(cmd.Context(), cmd, args)
}

func (c *CommandBundle) CreateActionStorer(ctx context.Context, flushLogs func()) *tasks.Storer {
	return tasks.NewStorer(ctx, c.bundler, c.bundle, tasks.StorerWithFlushLogs(flushLogs))
}

// WriteError serializes an error with an optional stack trace in the bundle.
func (c *CommandBundle) WriteError(ctx context.Context, err error) error {
	return c.bundle.WriteError(ctx, err)
}

// FlushWithExitInfo writes memory stats of the command, serialized docker info output, and
// other diagnostic information before flushing the bundle.
func (c *CommandBundle) FlushWithExitInfo(ctx context.Context) error {
	if err := c.bundle.WriteMemStats(ctx); err != nil {
		return err
	}

	client, err := docker.NewClient()
	if err != nil {
		return err
	}

	dockerInfo, err := client.Info(ctx)
	if err != nil {
		return err
	}
	if err := c.bundle.WriteDockerInfo(ctx, &dockerInfo); err != nil {
		return err
	}

	return c.bundler.Flush(ctx, c.bundle)
}

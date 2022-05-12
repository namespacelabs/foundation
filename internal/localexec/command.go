// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package localexec

import (
	"context"
	"io"
	"os"
	"os/exec"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Command struct {
	Label         string
	Command       string
	Dir           string
	Args          []string
	AdditionalEnv []string

	Persistent bool // Set to true if this is a persistent process.
}

func (c Command) Run(ctx context.Context) error {
	ev := tasks.Action("local.exec")
	if c.Persistent {
		ev = ev.Indefinite()
	} else {
		ev = ev.LogLevel(2)
	}

	return ev.Arg("command", c.Command).Arg("args", c.Args).Run(ctx, func(ctx context.Context) error {
		out := console.Output(ctx, c.label())

		stdout := io.MultiWriter(out,
			tasks.Attachments(ctx).Output(tasks.Output("stdout", "text/plain")),
		)

		const stderrOutputName = "stderr"
		stderr := io.MultiWriter(out,
			tasks.Attachments(ctx).Output(tasks.Output(stderrOutputName, "text/plain")),
		)

		cmd := exec.CommandContext(ctx, c.Command, c.Args...)
		cmd.Dir = c.Dir
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.Env = append(os.Environ(), c.AdditionalEnv...)

		if err := RunAndPropagateCancelation(ctx, c.label(), cmd); err != nil {
			// TODO consider passing an ID of a buffer instead of a callback.
			readerF := func() io.Reader { return tasks.Attachments(ctx).ReaderByName(stderrOutputName) }
			return fnerrors.CommandError(readerF, "invocation failed: %w", err)
		}
		return nil
	})
}

func (c Command) label() string {
	if c.Label != "" {
		return c.Label
	}
	return c.Command
}

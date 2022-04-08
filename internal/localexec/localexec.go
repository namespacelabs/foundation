// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package localexec

import (
	"context"
	"os/exec"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type RunOpts struct {
	OnStart func()
}

func RunAndPropagateCancelation(ctx context.Context, label string, cmd *exec.Cmd) error {
	return RunAndPropagateCancelationWithOpts(ctx, label, cmd, RunOpts{})
}

func RunAndPropagateCancelationWithOpts(ctx context.Context, label string, cmd *exec.Cmd, opts RunOpts) error {
	if err := checkCancelation(ctx, label, "execution start", cmd.Start()); err != nil {
		return err
	}

	if opts.OnStart != nil {
		opts.OnStart()
	}

	return WaitAndPropagateCancelation(ctx, label, cmd)
}

func WaitAndPropagateCancelation(ctx context.Context, label string, cmd *exec.Cmd) error {
	// When a context is canceled, os/exec will kill the child process. Often
	// this is surfaced as a "signal: killed" error, without more information.
	// Which makes it difficult to understand the actual reason. So instead
	// we check if the context was canceled, and return that instead.

	return checkCancelation(ctx, label, "execution", cmd.Wait())
}

func checkCancelation(ctx context.Context, label, what string, err error) error {
	if err == nil {
		return nil
	}

	if ctx.Err() != nil {
		err = ctx.Err()
	}

	return fnerrors.UserError(nil, "%s: local %s failed: %w", label, what, err)
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package executor

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"
)

type Executor interface {
	Go(func(context.Context) error)
	GoCancelable(func(context.Context) error) func()
}

func New(ctx context.Context) (Executor, func() error) {
	eg, ctx := errgroup.WithContext(ctx)
	return fromErrGroup(eg, ctx), eg.Wait
}

func fromErrGroup(eg *errgroup.Group, ctx context.Context) Executor {
	return &errGroupExecutor{eg, ctx}
}

type errGroupExecutor struct {
	eg  *errgroup.Group
	ctx context.Context
}

func (exec *errGroupExecutor) Go(f func(context.Context) error) {
	exec.eg.Go(func() error {
		return f(exec.ctx)
	})
}

func (exec *errGroupExecutor) GoCancelable(f func(context.Context) error) func() {
	ctxWithCancel, cancel := context.WithCancel(exec.ctx)
	exec.eg.Go(func() error {
		if err := f(ctxWithCancel); err != nil {
			// Don't let individual cancelation lead to the cancelation of the whole group.
			if !errors.Is(err, context.Canceled) {
				return err
			}
		}
		return nil
	})
	return cancel
}
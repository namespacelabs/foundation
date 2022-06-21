// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package executor

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"
)

type Executor interface {
	Go(func(context.Context) error)
	GoCancelable(func(context.Context) error) func()
}

func New(ctx context.Context, name string) (Executor, func() error) {
	eg, ctx := errgroup.WithContext(ctx)
	wait := func() error {
		if err := eg.Wait(); err != nil {
			return fmt.Errorf("%s: failed: %w", name, err)
		}
		return nil
	}
	return fromErrGroup(eg, ctx), wait
}

func Newf(ctx context.Context, format string, args ...interface{}) (Executor, func() error) {
	return New(ctx, fmt.Sprintf(format, args...))
}

func Serial(ctx context.Context) (Executor, func() error) {
	s := &serial{ctx: ctx}
	return s, func() error { return s.err }
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

type serial struct {
	ctx context.Context
	err error
}

func (s *serial) Go(f func(context.Context) error) {
	if s.err == nil {
		s.err = f(s.ctx)
	}
}

func (s *serial) GoCancelable(f func(context.Context) error) func() {
	if s.err == nil {
		s.err = f(s.ctx)
	}
	return func() {}
}

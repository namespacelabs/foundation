// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package executor

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"namespacelabs.dev/go-ids"
)

type Executor interface {
	Go(func(context.Context) error)
	GoCancelable(func(context.Context) error) func()
	Wait() error
}

func New(ctx context.Context, name string) (Executor, func() error) {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	exec := &errGroupExecutor{ctx: ctxWithCancel, cancel: cancel, name: name, id: ids.NewRandomBase32ID(8)}
	return exec, exec.Wait
}

func Newf(ctx context.Context, format string, args ...interface{}) (Executor, func() error) {
	return New(ctx, fmt.Sprintf(format, args...))
}

func Serial(ctx context.Context) (Executor, func() error) {
	s := &serial{ctx: ctx}
	return s, s.Wait
}

type errGroupExecutor struct {
	ctx    context.Context
	cancel func()
	name   string
	id     string

	wg sync.WaitGroup

	errOnce sync.Once
	err     error
}

func (exec *errGroupExecutor) Wait() error {
	exec.wg.Wait()
	exec.cancel()
	return exec.err
}

func (exec *errGroupExecutor) lowlevelGo(f func() error) {
	exec.wg.Add(1)

	go func() {
		defer exec.wg.Done()

		if err := f(); err != nil {
			exec.errOnce.Do(func() {
				exec.err = err
				exec.cancel()
			})
		}
	}()
}

func (exec *errGroupExecutor) Go(f func(context.Context) error) {
	exec.lowlevelGo(func() error {
		return f(exec.ctx)
	})
}

func (exec *errGroupExecutor) GoCancelable(f func(context.Context) error) func() {
	ctxWithCancel, cancel := context.WithCancel(exec.ctx)
	exec.lowlevelGo(func() error {
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

func (s *serial) Wait() error { return s.err }

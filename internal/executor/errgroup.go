// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package executor

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"namespacelabs.dev/go-ids"
)

type ExecutorLike interface {
	Go(func(context.Context) error)
	GoCancelable(func(context.Context) error) func()
	Wait() error
}

func New(ctx context.Context, name string) *Executor {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	return &Executor{ctx: ctxWithCancel, cancel: cancel, name: name, id: ids.NewRandomBase32ID(8)}
}

func Newf(ctx context.Context, format string, args ...interface{}) *Executor {
	return New(ctx, fmt.Sprintf(format, args...))
}

func NewSerial(ctx context.Context) *Serial {
	return &Serial{ctx: ctx}
}

type Executor struct {
	ctx    context.Context
	cancel func()
	name   string
	id     string

	wg sync.WaitGroup

	errOnce sync.Once
	err     error
}

func (exec *Executor) Wait() error {
	exec.wg.Wait()
	exec.cancel()
	return exec.err
}

func (exec *Executor) lowlevelGo(f func() error) {
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

func (exec *Executor) Go(f func(context.Context) error) {
	exec.lowlevelGo(func() error {
		return f(exec.ctx)
	})
}

func (exec *Executor) GoCancelable(f func(context.Context) error) func() {
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

type Serial struct {
	ctx context.Context
	err error
}

func (s *Serial) Go(f func(context.Context) error) {
	if s.err == nil {
		s.err = f(s.ctx)
	}
}

func (s *Serial) GoCancelable(f func(context.Context) error) func() {
	if s.err == nil {
		s.err = f(s.ctx)
	}
	return func() {}
}

func (s *Serial) Wait() error { return s.err }

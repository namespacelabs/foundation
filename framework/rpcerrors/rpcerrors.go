// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package rpcerrors

import (
	"fmt"
	"runtime"

	"github.com/go-errors/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Error struct {
	Err    error
	code   codes.Code
	stack  []uintptr
	frames []errors.StackFrame
}

func Wrap(code codes.Code, err error) *Error {
	return WrapWithSkip(code, err, 1)
}

func WrapWithSkip(code codes.Code, err error, skip int) *Error {
	stack := make([]uintptr, errors.MaxStackDepth)
	length := runtime.Callers(1+skip, stack[:])

	return &Error{
		Err:   err,
		code:  code,
		stack: stack[:length],
	}
}

func Errorf(code codes.Code, format string, args ...interface{}) *Error {
	stack := make([]uintptr, errors.MaxStackDepth)
	length := runtime.Callers(2, stack[:])
	return &Error{
		Err:   fmt.Errorf(format, args...),
		code:  code,
		stack: stack[:length],
	}
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) GRPCStatus() *status.Status {
	return status.New(e.code, e.Error())
}

func (err *Error) StackFrames() []errors.StackFrame {
	if err.frames == nil {
		err.frames = make([]errors.StackFrame, len(err.stack))

		for i, pc := range err.stack {
			err.frames[i] = errors.NewStackFrame(pc)
		}
	}

	return err.frames
}

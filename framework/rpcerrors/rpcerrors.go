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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Error struct {
	SafeMsg       string
	Err           error
	Code          codes.Code
	stack         []uintptr
	stkframecache []errors.StackFrame
	Details       []proto.Message
}

func Wrap(code codes.Code, err error) *Error {
	return WrapWithSkip(code, err, 1)
}

func WrapWithSkip(code codes.Code, err error, skip int) *Error {
	stack := make([]uintptr, errors.MaxStackDepth)
	length := runtime.Callers(1+skip, stack[:])

	return &Error{
		Err:   err,
		stack: stack[:length],
	}
}

func Errorf(code codes.Code, format string, args ...any) *Error {
	stack := make([]uintptr, errors.MaxStackDepth)
	length := runtime.Callers(2, stack[:])
	err := fmt.Errorf(format, args...)
	return &Error{
		Err:   err,
		Code:  code,
		stack: stack[:length],
	}
}

func Safef(code codes.Code, original error, format string, args ...any) *Error {
	stack := make([]uintptr, errors.MaxStackDepth)
	length := runtime.Callers(2, stack[:])
	return &Error{
		SafeMsg: fmt.Sprintf(format, args...),
		Err:     original,
		Code:    code,
		stack:   stack[:length],
	}
}

func (e *Error) Error() string {
	if e.SafeMsg != "" {
		return fmt.Sprintf("%s: %v", e.SafeMsg, e.Err)
	}

	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) GRPCStatus() *status.Status {
	if len(e.Details) == 0 {
		return status.New(e.Code, e.Error())
	}

	p := status.New(e.Code, e.Error()).Proto()
	for _, detail := range e.Details {
		any, _ := anypb.New(detail)
		if any != nil {
			p.Details = append(p.Details, any)
		}
	}

	return status.FromProto(p)
}

func (e *Error) WithDetails(details ...proto.Message) *Error {
	if e.Code == codes.OK {
		return e
	}

	return &Error{
		SafeMsg: e.SafeMsg,
		Err:     e.Err,
		Code:    e.Code,
		Details: append(e.Details, details...),
		stack:   e.stack,
	}
}

func (err *Error) StackFrames() []errors.StackFrame {
	if err.stkframecache == nil {
		err.stkframecache = make([]errors.StackFrame, len(err.stack))

		for i, pc := range err.stack {
			err.stkframecache[i] = errors.NewStackFrame(pc)
		}
	}

	return err.stkframecache
}

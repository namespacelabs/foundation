// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package rpcerrors

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	nsclouderrors "namespacelabs.dev/foundation/public/nscloud/proto/v1"
)

type Error struct {
	SafeMsg string
	Err     error
	Code    codes.Code
	Details []proto.Message
}

func Wrap(code codes.Code, err error) *Error {
	return WrapWithSkip(code, err, 1)
}

func WrapWithSkip(code codes.Code, err error, skip int) *Error {
	return &Error{
		Err:  err,
		Code: code,
	}
}

func Errorf(code codes.Code, format string, args ...any) *Error {
	err := fmt.Errorf(format, args...)
	return &Error{
		Err:  err,
		Code: code,
	}
}

func Safef(code codes.Code, original error, format string, args ...any) *Error {
	safeMsg := fmt.Sprintf(format, args...)
	return &Error{
		SafeMsg: safeMsg,
		Err:     original,
		Code:    code,
	}
}

func (e *Error) Error() string {
	if e.SafeMsg != "" {
		if e.Err == nil {
			return e.SafeMsg
		}

		return fmt.Sprintf("%s: %v", e.SafeMsg, e.Err)
	}

	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) GRPCStatus() *status.Status {
	if len(e.Details) == 0 && e.SafeMsg == "" {
		return status.New(e.Code, e.Error())
	}

	p := status.New(e.Code, e.Error()).Proto()
	if e.SafeMsg != "" {
		any, _ := anypb.New(&nsclouderrors.UserMessage{
			Message: e.SafeMsg,
		})
		if any != nil {
			p.Details = append(p.Details, any)
		}
	}

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
	}
}

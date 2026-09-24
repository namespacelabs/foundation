// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnerrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsOfKindEmbeddedBaseError(t *testing.T) {
	cause := errors.New("connection failed")
	err := InvocationError("registry", "push failed: %w", cause)

	if !IsOfKind(err, Kind_INVOCATION) {
		t.Fatal("InvocationError was not classified as an invocation error")
	}
	if IsOfKind(err, Kind_INTERNAL) {
		t.Fatal("InvocationError was classified as an internal error")
	}
	if !IsOfKind(fmt.Errorf("outer: %w", err), Kind_INVOCATION) {
		t.Fatal("wrapped InvocationError was not classified as an invocation error")
	}
	if !errors.Is(err, cause) {
		t.Fatal("InvocationError no longer unwraps to its cause")
	}
	if got, want := err.Error(), "failed when calling registry: push failed: connection failed"; got != want {
		t.Fatalf("InvocationError formatting changed: got %q, want %q", got, want)
	}
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func TestMaybeAsPermanent(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{
			name:      "connection refused push",
			err:       pushInvocationError(syscall.ECONNREFUSED),
			permanent: false,
		},
		{
			name:      "host unreachable push",
			err:       pushInvocationError(syscall.EHOSTUNREACH),
			permanent: false,
		},
		{
			name:      "non-network invocation error",
			err:       fnerrors.InvocationError("registry", "failed to push image: denied"),
			permanent: true,
		},
		{
			name:      "unrelated network error",
			err:       &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)},
			permanent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maybeAsPermanent(tt.err)
			var permanent *backoff.PermanentError
			if isPermanent := errors.As(got, &permanent); isPermanent != tt.permanent {
				t.Fatalf("permanent = %t, want %t (error: %v)", isPermanent, tt.permanent, got)
			}
			if !errors.Is(got, tt.err) {
				t.Fatalf("result no longer unwraps to original error: %v", got)
			}
		})
	}
}

func pushInvocationError(errno syscall.Errno) error {
	pushErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: os.NewSyscallError("connect", errno),
	}
	return fnerrors.InvocationError("registry", "failed to push image %q: %w", "registry.example/repo:tag", pushErr)
}

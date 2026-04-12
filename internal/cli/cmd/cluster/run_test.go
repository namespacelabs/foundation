// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestRunCmdRejectsDocumentedPurposeWithOn(t *testing.T) {
	cmd := NewRunCmd()
	cmd.SetArgs([]string{
		"--image", "busybox:latest",
		"--on", "existing-instance",
		"--documented_purpose", "testing-purpose",
	})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() error = nil, want validation error")
	}

	if !strings.Contains(err.Error(), "--documented_purpose can only be set when creating an environment") {
		t.Fatalf("ExecuteContext() error = %q, want documented_purpose validation error", err)
	}
}

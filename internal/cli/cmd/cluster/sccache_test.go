// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"io"
	"strings"
	"testing"
)

func TestSetupSccacheCacheRequiresCacheName(t *testing.T) {
	cmd := newSetupSccacheCacheCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when --cache_name is missing")
	}

	const want = `required flag(s) "cache_name" not set`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %q", want, err.Error())
	}
}

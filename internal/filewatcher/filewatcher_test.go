// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package filewatcher

import "testing"

func TestLongestCommonPathPrefix(t *testing.T) {
	list := []string{
		"/path/to/server/file",
		"/path/to/service",
		"/path/to/shared/",
	}
	got := longestCommonPathPrefix(list)
	want := "/path/to"
	if got != want {
		t.Errorf("longestCommonPathPrefix%v: got %q, want %q", list, got, want)
	}
}

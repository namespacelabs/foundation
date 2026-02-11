// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import "testing"

func TestParseGitConfigKeyValue(t *testing.T) {
	tests := []struct {
		input     string
		wantOk    bool
		wantKey   string
		wantValue string
	}{
		{"submodule.foo.path src/foo", true, "submodule.foo.path", "src/foo"},
		{"submodule.foo.url git@github.com:org/repo.git", true, "submodule.foo.url", "git@github.com:org/repo.git"},
		{"key value with spaces", true, "key", "value with spaces"},
		{"nospace", false, "", ""},
		{"", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ok, key, value := parseGitConfigKeyValue(tt.input)
			if ok != tt.wantOk || key != tt.wantKey || value != tt.wantValue {
				t.Errorf("parseGitConfigKeyValue(%q) = (%v, %q, %q), want (%v, %q, %q)",
					tt.input, ok, key, value, tt.wantOk, tt.wantKey, tt.wantValue)
			}
		})
	}
}

func TestParseSubmoduleConfigKey(t *testing.T) {
	tests := []struct {
		input         string
		wantOk        bool
		wantConfigKey string
		wantAttrName  string
	}{
		{"submodule.mylib.path", true, "mylib", "path"},
		{"submodule.mylib.url", true, "mylib", "url"},
		{"submodule.some-project-1.9.path", true, "some-project-1.9", "path"},
		{"submodule.some-project-1.9.url", true, "some-project-1.9", "url"},
		{"submodule.lib-1.2.3-rc.1.path", true, "lib-1.2.3-rc.1", "path"},
		{"submodule.a.b.c.path", true, "a.b.c", "path"},
		{"other.foo.path", false, "", ""},
		{"submodule.path", false, "", ""},
		{"submodule..path", false, "", ""},
		{"submodule.foo.", false, "", ""},
		{"nodots", false, "", ""},
		{"", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ok, configKey, attrName := parseSubmoduleConfigKey(tt.input)
			if ok != tt.wantOk || configKey != tt.wantConfigKey || attrName != tt.wantAttrName {
				t.Errorf("parseSubmoduleConfigKey(%q) = (%v, %q, %q), want (%v, %q, %q)",
					tt.input, ok, configKey, attrName, tt.wantOk, tt.wantConfigKey, tt.wantAttrName)
			}
		})
	}
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"testing"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/assert"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestBazelTarget(t *testing.T) {
	target, err := bazelTarget(GoBinary{BazelPackagePath: "global/server/iam"})
	if err != nil {
		t.Fatal(err)
	}
	if target != "//global/server/iam:iam" {
		t.Fatalf("target = %q", target)
	}

	target, err = bazelTarget(GoBinary{BazelPackagePath: ".", GoModule: "example.com/acme/tool"})
	if err != nil {
		t.Fatal(err)
	}
	if target != "//:tool" {
		t.Fatalf("root target = %q", target)
	}
}

func TestRulesGoPlatform(t *testing.T) {
	label, err := rulesGoPlatform(specs.Platform{OS: "linux", Architecture: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if label != "@rules_go//go/toolchain:linux_amd64" {
		t.Fatalf("label = %q", label)
	}

	if _, err := rulesGoPlatform(specs.Platform{OS: "linux", Architecture: "arm", Variant: "v7"}); err == nil {
		t.Fatal("expected variant to be rejected")
	}
}

func TestGoModuleVersion(t *testing.T) {
	mod := pkggraph.NewModule(&schema.Workspace{ModuleName: "example.com/acme"}, nil, "0123456789ab")
	assert.Equal(t, "0123456789ab", goModuleVersion(mod, "example.com/acme"))
	assert.Equal(t, goModuleVersion(mod, "example.com/acme/nested"), "")
}

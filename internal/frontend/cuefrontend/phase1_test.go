// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestEnsureStartupEnvSecretPackagesLoaded(t *testing.T) {
	ctx := context.Background()
	loc := pkggraph.Location{PackageName: "example.com/current/service"}
	v := cuecontext.New().CompileString(`{
	STATIC: "value"
	LOCAL: fromSecret: ":local"
	REMOTE: fromSecret: "example.com/shared/secrets:password"
}`)
	if err := v.Err(); err != nil {
		t.Fatal(err)
	}

	loader := &recordingPackageLoader{}
	if err := ensureStartupEnvSecretPackagesLoaded(ctx, loader, loc, v); err != nil {
		t.Fatal(err)
	}

	if len(loader.ensured) != 1 || loader.ensured[0] != "example.com/shared/secrets" {
		t.Fatalf("expected only remote secret package to be ensured, got %v", loader.ensured)
	}
}

type recordingPackageLoader struct {
	ensured []schema.PackageName
}

func (r *recordingPackageLoader) Resolve(context.Context, schema.PackageName) (pkggraph.Location, error) {
	panic("unexpected Resolve")
}

func (r *recordingPackageLoader) LoadByName(context.Context, schema.PackageName) (*pkggraph.Package, error) {
	panic("unexpected LoadByName")
}

func (r *recordingPackageLoader) Ensure(_ context.Context, packageName schema.PackageName) error {
	r.ensured = append(r.ensured, packageName)
	return nil
}

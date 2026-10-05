// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestParseContainerResources(t *testing.T) {
	for _, input := range []string{
		`{vector: {binary: "example.com/vector", args: {}, resourceRequests: {cpu: "1", memory: "4Gi"}, resourceLimits: {cpu: "2", memory: "6Gi"}}}`,
		`[{name: "vector", binary: "example.com/vector", args: {}, resourceRequests: {cpu: "1", memory: "4Gi"}, resourceLimits: {cpu: "2", memory: "6Gi"}}]`,
	} {
		v := cuecontext.New().CompileString(input)
		containers, err := parseContainers(pkggraph.Location{PackageName: "example.com/server"}, "sidecar", v)
		if !assert.NoError(t, err) || !assert.Len(t, containers, 1) {
			continue
		}
		assert.Equal(t, &schema.Container_ResourceLimits{Cpu: "1", Memory: "4Gi"}, containers[0].Requests)
		assert.Equal(t, &schema.Container_ResourceLimits{Cpu: "2", Memory: "6Gi"}, containers[0].Limits)
	}
}

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

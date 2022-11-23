// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"testing"

	"gotest.tools/assert"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
)

func TestPackagesFromArgs(t *testing.T) {
	opts := ParseLocationsOpts{}

	// Root cwd
	assertLocationFromArgs(t, ".", "servers/server1", "servers/server1", opts)
	assertLocationFromArgs(t, ".", "./servers/server1", "servers/server1", opts)

	// Non-root cwd
	assertLocationFromArgs(t, "servers", "server1", "servers/server1", opts)
	assertLocationFromArgs(t, "servers", "./server1", "servers/server1", opts)
	assertLocationFromArgs(t, "servers", "../extentions/ext1", "extentions/ext1", opts)
	assertLocationFromArgs(t, "servers/server1", "..", "servers", opts)

	// Fully-qualified package name
	assertLocationFromArgs(t, ".", "github.com/myuser/mymodule/servers/server1", "servers/server1", opts)
	assertLocationFromArgs(t, "servers", "github.com/myuser/mymodule/servers/server1", "servers/server1", opts)
	assertLocationFromArgs(t, "services", "github.com/myuser/mymodule/servers/server1", "servers/server1", opts)
	assertLocationFromArgsForModule(t, "services", "namespacelabs.com/myuser2/othermodule/servers/server1",
		"servers/server1", "namespacelabs.com/myuser2/othermodule", opts)

	assertLocationFromArgs(t, ".", "servers/...", "servers/server1", opts)

	// Error cases
	_, _, err := locationsAndPackageRefsFromArgs(context.Background(), moduleName, allModules, "servers", []string{"/abs/path"}, nil, opts)
	assert.ErrorContains(t, err, "absolute paths are not supported")

	_, _, err = locationsAndPackageRefsFromArgs(context.Background(), moduleName, allModules, "servers", []string{"../../othermodule"}, nil, opts)
	assert.ErrorContains(t, err, "can't refer to packages outside of the module root")
}

const moduleName = "github.com/myuser/mymodule"

var allModules = []string{moduleName, "namespacelabs.com/myuser2/othermodule"}

func assertLocationFromArgs(t *testing.T, relCwd string, arg string, expectedRelPath string, opts ParseLocationsOpts) {
	assertLocationFromArgsForModule(t, relCwd, arg, expectedRelPath, moduleName, opts)
}

func assertLocationFromArgsForModule(t *testing.T, relCwd string, arg string, expectedRelPath string, expectedModuleName string, opts ParseLocationsOpts) {
	locations, _, err := locationsAndPackageRefsFromArgs(context.Background(), moduleName, allModules, relCwd, []string{arg}, func() (parsing.SchemaList, error) {
		var p parsing.SchemaList
		p.Locations = append(p.Locations, fnfs.Location{
			ModuleName: moduleName,
			RelPath:    "servers/server1",
		})
		return p, nil
	}, opts)
	assert.NilError(t, err)
	assert.Equal(t, len(locations), 1)
	assert.Equal(t, locations[0].ModuleName, expectedModuleName)
	if locations[0].RelPath != expectedRelPath {
		t.Errorf("relCwd: %q, arg: %q. Got rel path %q, expected %q.", relCwd, arg, locations[0].RelPath, expectedRelPath)
	}
}

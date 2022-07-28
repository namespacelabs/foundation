// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"testing"

	"gotest.tools/assert"
)

func TestPackagesFromArgs(t *testing.T) {
	// Root cwd
	assertLocationFromArgs(t, ".", "servers/server1", "servers/server1")
	assertLocationFromArgs(t, ".", "./servers/server1", "servers/server1")

	// Non-root cwd
	assertLocationFromArgs(t, "servers", "server1", "servers/server1")
	assertLocationFromArgs(t, "servers", "./server1", "servers/server1")
	assertLocationFromArgs(t, "servers", "../extentions/ext1", "extentions/ext1")
	assertLocationFromArgs(t, "servers/server1", "..", "servers")

	// Fully-qualified package name
	assertLocationFromArgs(t, ".", "github.com/myuser/mymodule/servers/server1", "servers/server1")
	assertLocationFromArgs(t, "servers", "github.com/myuser/mymodule/servers/server1", "servers/server1")
	assertLocationFromArgs(t, "services", "github.com/myuser/mymodule/servers/server1", "servers/server1")

	// Error cases
	_, err := locationsFromArgs(moduleName, "servers", []string{"/abs/path"})
	assert.ErrorContains(t, err, "absolute paths are not supported")

	_, err = locationsFromArgs(moduleName, "servers", []string{"../../othermodule"})
	assert.ErrorContains(t, err, "can't refer to packages outside of the module root")
}

const moduleName = "github.com/myuser/mymodule"

func assertLocationFromArgs(t *testing.T, relCwd string, arg string, expectedRelPath string) {
	locations, err := locationsFromArgs(moduleName, relCwd, []string{arg})
	assert.NilError(t, err)
	assert.Equal(t, len(locations), 1)
	assert.Equal(t, locations[0].ModuleName, moduleName)
	if locations[0].RelPath != expectedRelPath {
		t.Errorf("relCwd: %q, arg: %q. Got rel path %q, expected %q.", relCwd, arg, locations[0].RelPath, expectedRelPath)
	}
}

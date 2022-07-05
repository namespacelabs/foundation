// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cue

import (
	"context"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
)

const (
	testFileName = "test.cue"
)

type GenTestOpts struct {
	ServerPkg string
}

func CreateTestScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenTestOpts) error {
	return generateCueSource(ctx, fsfs, loc.Rel(testFileName), testTmpl, opts)
}

var testTmpl = template.Must(template.New(testFileName).Parse(`
import "namespacelabs.dev/foundation/std/fn"

// Declare a new test, see also https://docs.namespacelabs.com/concepts/test
test: fn.#Test & {
	name: "e2etest"

	// Define how the test driver gets built.
	binary: {
		// In this case, the test driver is built from a go binary which is co-located with the test.
		from: go_package: "."
	}

	fixture: {
		// The server under test. Its dependencies will be automatically part of the test fixture.
		sut: "{{.ServerPkg}}"
	}
}
`))

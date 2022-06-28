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

test: fn.#Test & {
	name: "e2etest"

	binary: {
		from: go_package: "."
	}

	fixture: {
		sut: "{{.ServerPkg}}"
	}
}
`))

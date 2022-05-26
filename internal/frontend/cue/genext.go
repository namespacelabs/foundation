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
	extensionFileName = "extension.cue"
)

func CreateExtensionScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location) error {
	opts := extensionTmplOptions{}

	return generateCueSource(ctx, fsfs, loc.Rel(extensionFileName), extensionTmpl, opts)
}

type extensionTmplOptions struct{}

var extensionTmpl = template.Must(template.New(extensionFileName).Parse(`
import (
	"namespacelabs.dev/foundation/std/fn"
)

// For examples, check out namespacelabs.dev/std and namespacelabs.dev/universe
extension: fn.#Extension & {

}
`))

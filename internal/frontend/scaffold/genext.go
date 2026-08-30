// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package scaffold

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

extension: fn.#Extension & {

}
`))

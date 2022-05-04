// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cue

import (
	"context"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
)

const (
	serviceFileName = "service.cue"
)

func GenerateService(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, name string, framework schema.Framework) error {
	opts := serviceTmplOptions{
		Name:      name,
		Framework: framework.String(),
	}

	return generateCueSource(ctx, fsfs, loc.Rel(serviceFileName), serviceTmpl, opts)
}

type serviceTmplOptions struct {
	Name      string
	Framework string
}

var serviceTmpl = template.Must(template.New(serviceFileName).Parse(`
import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
	framework: "{{.Framework}}"

	exportService: $proto.services.{{.Name}}
}
`))

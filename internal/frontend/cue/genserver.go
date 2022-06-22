// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cue

import (
	"context"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/go-ids"
)

const (
	serverFileName = "server.cue"
)

type GenServerOpts struct {
	Name      string
	Framework schema.Framework
}

func CreateServerScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenServerOpts) error {
	return generateCueSource(ctx, fsfs, loc.Rel(serverFileName), serverTmpl, serverTmplOptions{
		Id:        ids.NewRandomBase32ID(12),
		Name:      opts.Name,
		Framework: opts.Framework.String(),
	})
}

type serverTmplOptions struct {
	Id        string
	Name      string
	Framework string
}

var serverTmpl = template.Must(template.New(serverFileName).Parse(`
import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "{{.Id}}"
	name:      "{{.Name}}"
	framework: "{{.Framework}}"

	import: [
		// TODO add services here
	]
}
`))

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
	nodeFileName = "server.cue"
)

func GenerateServer(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, name string, framework schema.Framework) error {
	opts := serverTmplOptions{
		Id:        ids.NewRandomBase32ID(12),
		Name:      name,
		Framework: framework.String(),
	}

	return generateCueSource(ctx, fsfs, loc.Rel(nodeFileName), serverTmpl, opts)
}

type serverTmplOptions struct {
	Id        string
	Name      string
	Framework string
}

var serverTmpl = template.Must(template.New(nodeFileName).Parse(`
import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "{{.Id}}"
	name:      "{{.Name}}"
	framework: "{{.Framework}}"

	import: [
		{{- if eq .Framework "GO_GRPC"}}
		// To expose GRPC endpoints via HTTP, add this import: 
		// "namespacelabs.dev/foundation/std/go/grpc/gateway",
		{{end}}
		// TODO add services here
	]
}
`))

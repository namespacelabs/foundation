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
	Name         string
	Framework    schema.Framework
	GrpcServices []string
	HttpServices []HttpService
}

func CreateServerScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenServerOpts) error {
	return generateCueSource(ctx, fsfs, loc.Rel(serverFileName), serverTmpl, serverTmplOptions{
		Id:           ids.NewRandomBase32ID(12),
		Name:         opts.Name,
		Framework:    opts.Framework.String(),
		GrpcServices: opts.GrpcServices,
		HttpServices: opts.HttpServices,
	})
}

type serverTmplOptions struct {
	Id           string
	Name         string
	Framework    string
	GrpcServices []string
	HttpServices []HttpService
}

type HttpService struct {
	Path string
	Pkg  string
}

var serverTmpl = template.Must(template.New(serverFileName).Parse(`
import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "{{.Id}}"
	name:      "{{.Name}}"
	framework: "{{.Framework}}"

	{{- if .GrpcServices}}

	import: [
		{{- range .GrpcServices}}
		"{{.}}",
		{{- end}}
		// Add more gRPC services here
	]
	{{- end}}

	{{- if .HttpServices}}

	urlmap: [
		{{- range .HttpServices}}
		{path: "{{.Path}}", import: "{{.Pkg}}"},
		{{- end}}
	]
	{{- end}}
}
`))

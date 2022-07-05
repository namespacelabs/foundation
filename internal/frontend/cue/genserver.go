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
	Dependencies []string
	HttpServices []HttpService
}

func CreateServerScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenServerOpts) error {
	return generateCueSource(ctx, fsfs, loc.Rel(serverFileName), serverTmpl, serverTmplOptions{
		Id:              ids.NewRandomBase32ID(12),
		Name:            opts.Name,
		Framework:       opts.Framework.String(),
		GrpcServices:    opts.GrpcServices,
		Dependencies:    opts.Dependencies,
		HasDependencies: (len(opts.GrpcServices) + len(opts.Dependencies)) > 0,
		HttpServices:    opts.HttpServices,
	})
}

type serverTmplOptions struct {
	Id              string
	Name            string
	Framework       string
	GrpcServices    []string
	Dependencies    []string
	HasDependencies bool
	HttpServices    []HttpService
}

type HttpService struct {
	Path string
	Pkg  string
}

var serverTmpl = template.Must(template.New(serverFileName).Parse(`
import (
	"namespacelabs.dev/foundation/std/fn"
)

// Declare a new server, see also https://docs.namespacelabs.com/concepts/server
server: fn.#Server & {
	// Each server has a unique ID, which persists package moves. This allows Namespace
	// to coordinate production changes during code refactors.
	id:        "{{.Id}}"

	// The name of the server is used primarily for debugging purposes, in places where
	// the use of the full package name is not practical.
	name:      "{{.Name}}"

	// The language/framework combo this server supports.
	framework: "{{.Framework}}"

	{{- if .HasDependencies}}

	// Imports declare what gets composed into this server. They're often a combination of
	// services and other functionality that gets applied server-wide, exported by extensions.
	import: [
		{{- range .GrpcServices}}
		"{{.}}",
		{{- end}}
		// Add more gRPC services here
		{{- range .Dependencies}}
		"{{.}}",
		{{- end}}
	]
	{{- end}}

	{{- if .HttpServices}}

	// Specifies which URL paths map to what web UIs.
	urlmap: [
		{{- range .HttpServices}}
		{path: "{{.Path}}", import: "{{.Pkg}}"},
		{{- end}}
	]
	{{- end}}
}
`))

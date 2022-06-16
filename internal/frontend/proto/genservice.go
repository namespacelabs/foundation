// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package proto

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
)

const (
	serviceFileName = "service.proto"
)

type GenServiceOpts struct {
	Name      string
	Framework schema.Framework
}

func CreateProtoScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenServiceOpts) error {
	parts := strings.Split(loc.RelPath, string(os.PathSeparator))

	return createProtoScaffold(ctx, fsfs, loc.Rel(serviceFileName), serviceTmpl, serviceTmplOptions{
		Name:      opts.Name,
		Package:   strings.Join(parts, "."),
		GoPackage: filepath.Join(append([]string{loc.ModuleName}, parts...)...),
		Framework: opts.Framework.String(),
	})
}

type serviceTmplOptions struct {
	Name      string
	Package   string
	GoPackage string
	Framework string
}

// TODO clean up template once we run a proto formatter.
var serviceTmpl = template.Must(template.New(serviceFileName).Parse(`syntax = "proto3";

package {{.Package}};{{if eq .Framework "GO"}}

option go_package = "{{.GoPackage}}";{{end}}

service {{.Name}} {
    // TODO change to desired RPC methods
	rpc Echo(EchoRequest) returns (EchoResponse);
}

message EchoRequest {
	string text = 1;
}

message EchoResponse {
	string text = 1;
}

`))

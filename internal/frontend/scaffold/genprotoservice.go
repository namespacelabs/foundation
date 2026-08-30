// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package scaffold

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
	protoServiceFileName = "service.proto"
)

type GenProtoServiceOpts struct {
	Name      string
	Framework schema.Framework
}

func CreateProtoScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenProtoServiceOpts) error {
	parts := strings.Split(loc.RelPath, string(os.PathSeparator))

	return createProtoScaffold(ctx, fsfs, loc.Rel(protoServiceFileName), protoServiceTmpl, protoServiceTmplOptions{
		Name:      opts.Name,
		Package:   strings.Join(parts, "."),
		GoPackage: filepath.Join(append([]string{loc.ModuleName}, parts...)...),
		Framework: opts.Framework.String(),
	})
}

type protoServiceTmplOptions struct {
	Name      string
	Package   string
	GoPackage string
	Framework string
}

// TODO clean up template once we run a proto formatter.
var protoServiceTmpl = template.Must(template.New(protoServiceFileName).Parse(`syntax = "proto3";

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

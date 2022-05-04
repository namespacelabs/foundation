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

func GenerateService(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, name string, framework schema.Framework) error {
	parts := strings.Split(loc.RelPath, string(os.PathSeparator))

	opts := serviceTmplOptions{
		Name:      name,
		Package:   strings.Join(parts, "."),
		GoPackage: filepath.Join(append([]string{loc.ModuleName}, parts...)...),
		Framework: framework.String(),
	}

	return generateProtoSource(ctx, fsfs, loc.Rel(serviceFileName), serviceTmpl, opts)
}

type serviceTmplOptions struct {
	Name      string
	Package   string
	GoPackage string
	Framework string
}

// TODO clean up template once we run a proto formatter.
var serviceTmpl = template.Must(template.New(serviceFileName).Parse(`syntax = "proto3";

package {{.Package}};{{if eq .Framework "GO_GRPC"}}

option go_package = "{{.GoPackage}}";{{end}}

service {{.Name}} {
    // TODO add RPCs here
}
`))

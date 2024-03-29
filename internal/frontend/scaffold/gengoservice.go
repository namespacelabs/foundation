// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package scaffold

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
)

const (
	goImplFileName = "impl.go"
)

type GenGoServiceOpts struct {
	Name string
}

func CreateGoServiceScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenGoServiceOpts) error {
	parts := strings.Split(loc.RelPath, string(os.PathSeparator))

	if len(parts) < 1 {
		return fmt.Errorf("unable to determine package name")
	}

	return generateGoSource(ctx, fsfs, loc.Rel(goImplFileName), goServiceTmpl, goServiceTmplOptions{
		Name:    opts.Name,
		Package: parts[len(parts)-1],
	})
}

type goServiceTmplOptions struct {
	Name    string
	Package string
}

var goServiceTmpl = template.Must(template.New(goImplFileName).Parse(`
package {{.Package}}

import (
	"context"

	"namespacelabs.dev/foundation/std/go/server"
)

type Service struct {
}

func (svc *Service) Echo(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	// TODO add business logic here
	return &EchoResponse{ Text: req.Text }, nil
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{}
	Register{{.Name}}Server(srv, svc)
}

`))

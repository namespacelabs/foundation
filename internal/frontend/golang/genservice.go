// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
)

const (
	implFileName = "impl.go"
)

type GenServiceOpts struct {
	Name string
}

func CreateServiceScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenServiceOpts) error {
	parts := strings.Split(loc.RelPath, string(os.PathSeparator))

	if len(parts) < 1 {
		return fmt.Errorf("unable to determine package name")
	}

	return generateGoSource(ctx, fsfs, loc.Rel(implFileName), serviceTmpl, serviceTmplOptions{
		Name:    opts.Name,
		Package: parts[len(parts)-1],
	})
}

type serviceTmplOptions struct {
	Name    string
	Package string
}

var serviceTmpl = template.Must(template.New(implFileName).Parse(`
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

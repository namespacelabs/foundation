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
	goTestFileName = "e2etest.go"
)

type GenGoTestOpts struct {
	ServicePkg string
}

func CreateGoTestScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenGoTestOpts) error {
	serverPackageParts := strings.Split(opts.ServicePkg, string(os.PathSeparator))
	if len(serverPackageParts) < 1 {
		return fmt.Errorf("unable to determine server package name")
	}

	return generateGoSource(ctx, fsfs, loc.Rel(goTestFileName), goTestTmpl, goTestTmplOptions{
		ServicePkg:         opts.ServicePkg,
		ServiceImportAlias: serverPackageParts[len(serverPackageParts)-1],
	})
}

type goTestTmplOptions struct {
	ServicePkg         string
	ServiceImportAlias string
}

var goTestTmpl = template.Must(template.New(goImplFileName).Parse(`
package main

import (
	"context"
	"fmt"

	"{{.ServicePkg}}"
	"namespacelabs.dev/foundation/framework/testing"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		conn, err := t.Connect(ctx, t.MustEndpoint("{{.ServicePkg}}", "{{.ServiceImportAlias}}"))
		if err != nil {
			return err
		}

		client := {{.ServiceImportAlias}}.NewEchoServiceClient(conn)

		req := {{.ServiceImportAlias}}.EchoRequest{
			Text: "Hello, World!",
		}

		response, err := client.Echo(ctx, &req)
		if err != nil {
			return err
		}

		if response.Text != req.Text {
			return fmt.Errorf("expected %s, got %s", req.Text, response.Text)
		}

		return nil
	})
}
`))

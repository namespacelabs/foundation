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
	testFileName = "e2etest.go"
)

type GenTestOpts struct {
	ServicePkg string
}

func CreateTestScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenTestOpts) error {
	serverPackageParts := strings.Split(opts.ServicePkg, string(os.PathSeparator))
	if len(serverPackageParts) < 1 {
		return fmt.Errorf("unable to determine server package name")
	}

	return generateGoSource(ctx, fsfs, loc.Rel(testFileName), testTmpl, testTmplOptions{
		ServicePkg:         opts.ServicePkg,
		ServiceImportAlias: serverPackageParts[len(serverPackageParts)-1],
	})
}

type testTmplOptions struct {
	ServicePkg         string
	ServiceImportAlias string
}

var testTmpl = template.Must(template.New(implFileName).Parse(`
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

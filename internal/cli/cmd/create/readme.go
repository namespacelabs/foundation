// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"

	"github.com/charmbracelet/glamour"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	readmeFilePath = "README.md"
)

var (
	readmeTemplate = template.Must(template.New("readme").Parse(`Your starter Namespace project has been generated!

{{if .RemoteUrl -}}
[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#{{.RemoteUrl}})

{{end -}}

Next steps:

{{if .Dir -}}
- Switch to the project directory: ` + "`" + `cd {{.Dir}}` + "`" + `
{{end -}}
- Run ` + "`" + `ns prepare local` + "`" + ` to prepare the local dev environment.
- Run ` + "`" + `ns test {{.TestPkg}}` + "`" + ` to run the e2e test.
- Run ` + "`" + `ns dev {{.ServerPkg}}` + "`" + ` to start the server stack in the development mode with hot reload.
`))
)

type readmeTmplOpts struct {
	Dir       string
	ServerPkg string
	TestPkg   string
	RemoteUrl string
}

type readmeOpts struct {
	Dir              string
	RepoUrlForGitpod string
}

func generateReadme(ctx context.Context, opts *readmeOpts) (string, error) {
	dir := opts.Dir
	// No need to change the directory if it's the current one.
	if dir == "." {
		dir = ""
	}
	data := readmeTmplOpts{
		Dir:       dir,
		ServerPkg: webServerPkg,
		TestPkg:   testPkg,
	}

	data.RemoteUrl = opts.RepoUrlForGitpod

	var body bytes.Buffer
	if err := readmeTemplate.Execute(&body, data); err != nil {
		return "", fnerrors.InternalError("failed to apply template: %w", err)
	}

	return body.String(), nil
}

func renderMarkdown(out io.Writer, str string) error {
	r, _ := glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithAutoStyle(),
	)
	rendered, err := r.Render(str)
	if err != nil {
		return err
	}
	fmt.Fprint(out, rendered)
	return nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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

var (
	readmeFileTemplate = template.Must(template.New("starterinfo").Parse(`# Welcome to Namespace!

This project has been generated by ` + "`" + `ns starter` + "`" + `. See the
[documentation](https://docs.namespacelabs.com/getting-started/) for more details.

{{- if .RemoteUrl}}

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#{{.RemoteUrl}})
{{end}}
`))
	starterInfoTemplate = template.Must(template.New("starterinfo").Parse(`# Welcome to Namespace!

Your starter project has been created!

{{if .DryRun -}}
This project has been generated by ` + "`" + `ns starter` + "`" + `. See above the individual commands that were executed.

{{end -}}

## Next steps

{{if .Dir -}}
- Switch to the project directory: ` + "`" + `cd {{.Dir}}` + "`" + `
{{end -}}
{{if .SuggestPrepare -}}
- Run ` + "`" + `ns prepare local` + "`" + ` to prepare the local dev environment.
{{end -}}
- Run ` + "`" + `ns test {{.TestPkg}}` + "`" + ` or just ` + "`" + `ns test` + "`" + ` to run the e2e tests.
- Run ` + "`" + `ns dev{{range .DevPkgs}} {{.}}{{ end }}` + "`" + ` to start the server stack in the development mode with hot reload.
- See the documentation for more details: https://docs.namespacelabs.com/getting-started/
`))
)

type starterInfoTmplOpts struct {
	Dir            string
	DevPkgs        []string
	TestPkg        string
	RemoteUrl      string
	DryRun         bool
	SuggestPrepare bool
}

type starterInfoOpts struct {
	Dir              string
	RepoUrlForGitpod string
	DryRun           bool
	SuggestPrepare   bool
}

func generateStarterInfo(ctx context.Context, t *template.Template, opts *starterInfoOpts) (string, error) {
	dir := opts.Dir
	// No need to change the directory if it's the current one.
	if dir == "." {
		dir = ""
	}
	data := starterInfoTmplOpts{
		Dir:            dir,
		DevPkgs:        []string{goServerPkg, webServerPkg},
		TestPkg:        testPkg,
		RemoteUrl:      opts.RepoUrlForGitpod,
		DryRun:         opts.DryRun,
		SuggestPrepare: opts.SuggestPrepare,
	}

	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		return "", fnerrors.InternalError("failed to apply template: %w", err)
	}

	return body.String(), nil
}

func renderMarkdown(out io.Writer, str string) error {
	r, _ := glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithStandardStyle("dracula"),
	)
	rendered, err := r.Render(str)
	if err != nil {
		return err
	}
	fmt.Fprint(out, rendered)
	return nil
}

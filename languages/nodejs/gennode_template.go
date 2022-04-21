// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type nodeTmplOptions struct {
	Imports        []singleImport
	NeedsDepsType  bool
	HasGrpcService bool
	Providers      []provider
}

type provider struct {
	Name       string
	InputType  importedType
	OutputType importedType
}

var (
	serviceTmpl = template.Must(template.New("template").Parse(
		`// This file was automatically generated.

{{if .HasGrpcService}}
import { Server } from "@grpc/grpc-js";
{{end}}
import * as impl from "./impl";

{{range .Imports}}
import * as {{.Alias}} from "{{.Package}}"{{end}}

{{if .NeedsDepsType}}
export interface Deps {
}
{{end}}

{{range $.Providers}}
export type Provide{{.Name}} = (input: {{.InputType.ImportAlias}}.{{.InputType.Name}}) =>
		{{.OutputType.ImportAlias}}.{{.OutputType.Name}};
export const provide{{.Name}}: Provide{{.Name}} = impl.provide{{.Name}};
{{end}}

{{if .HasGrpcService}}
export type WireService = (deps: Deps, server: Server) => void;
export const wireService: WireService = impl.wireService;
{{end}}
`))
)

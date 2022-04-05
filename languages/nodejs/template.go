// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type nodeTmplOptions struct {
	Imports       []singleImport
	NeedsDepsType bool
	DepVars       []depVar
}

type depVar struct {
	Type typeDef
	Name string
}

type typeDef struct {
	ImportAlias, Name string
}

type singleImport struct {
	Alias, Package string
}

var (
	serviceTmpl = template.Must(template.New(ServiceDepsFilename).Parse(
		`// This file was automatically generated.{{with $opts := .}}
import { Server } from "@grpc/grpc-js";
import * as wire from "./wire";
{{range $opts.Imports}}
import * as {{.Alias}} from "{{.Package}}"{{end}}

{{if .NeedsDepsType}}
export interface Deps {
{{range $k, $v := .DepVars}}
	{{$v.Name}}: {{if $v.Type.ImportAlias}}{{$v.Type.ImportAlias}}.{{end}}{{$v.Type.Name}}{{end}}
}
{{end}}

export type WireService = (deps: Deps, server: Server) => void;
export const wireService: WireService = wire.wireService;

{{end}}`))
)

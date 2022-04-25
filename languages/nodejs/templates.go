// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type nodeTmplOptions struct {
	Imports       []tmplSingleImport
	HasService    bool
	SingletonDeps *tmplDeps
	Providers     []tmplProvider
}
type serverTmplOptions struct {
	Imports  []tmplSingleImport
	Services []tmplImportedType
}

type nodeImplTmplOptions struct {
	ServiceServerName, ServiceName, ServiceFileName string
}

type tmplProvider struct {
	Name       string
	InputType  tmplImportedType
	OutputType tmplImportedType
	ScopedDeps *tmplDeps
}

type tmplDeps struct {
	Name string
	Deps []tmplDependency
}

type tmplDependency struct {
	Name              string
	Type              tmplImportedType
	Provider          tmplImportedType
	ProviderInputType tmplImportedType
	ProviderInput     tmplSerializedProto
	HasScopedDeps     bool
	HasSingletonDeps  bool
}
type tmplSerializedProto struct {
	Base64Content string
	Comments      []string
}

type tmplImportedType struct {
	ImportAlias, Name string
}

type tmplSingleImport struct {
	Alias, Package string
}

var (
	tmpl = template.Must(template.New("template").Parse(
		// Helper templates
		`
// Input: tmplDeps				
{{define "Deps"}}
export interface {{.Name}}Deps {
{{- range .Deps}}
	{{.Name}}: {{.Type.ImportAlias}}.{{.Type.Name}};
{{- end}}
}

export const make{{.Name}}Deps = (dg: DependencyGraph): {{.Name}}Deps => ({
	{{- range .Deps}}
	  {{- range .ProviderInput.Comments}}
		// {{.}}
		{{- end}}
		{{.Name}}: {{.Provider.ImportAlias}}.provide{{.Provider.Name}}(
			{{.ProviderInputType.ImportAlias}}.{{.ProviderInputType.Name}}.deserializeBinary(
				Buffer.from("{{.ProviderInput.Base64Content}}", "base64"))
		{{- if .HasSingletonDeps}},
		  {{.Provider.ImportAlias}}.make` + singletonNameBase + `Deps(dg)
		{{- end}}
		{{- if .HasScopedDeps}},
		  {{.Provider.ImportAlias}}.make{{.Provider.Name}}Deps(dg)
		{{- end}}),
	{{- end}}
});
{{- end}}

{{define "Imports"}}
{{range .Imports -}}
import * as {{.Alias}} from "{{.Package}}"
{{end}}
{{end}}` +

			// Node template
			`{{define "Node"}}{{with $opts := .}}// This file was automatically generated.

{{if .HasService}}
import { Server } from "@grpc/grpc-js";
{{- end}}
import * as impl from "./impl";
import { DependencyGraph } from "foundation-runtime";

{{- template "Imports" . -}}

{{if .SingletonDeps}}
{{- template "Deps" .SingletonDeps}}
{{- end}}

{{- if .HasService}}

export type WireService = (
	{{- if .SingletonDeps}}deps: {{.SingletonDeps.Name}}Deps, {{end -}}
	server: Server) => void;
export const wireService: WireService = impl.wireService;
{{- end}}

{{- range $.Providers -}}

{{if .ScopedDeps}}
// Scoped dependencies that are instantiated for each call to Provide{{.Name}}.
{{template "Deps" .ScopedDeps}}
{{- end}}

export type Provide{{.Name}} = (input: {{.InputType.ImportAlias}}.{{.InputType.Name}}
	  {{- if $opts.SingletonDeps}}, singletonDeps: {{$opts.SingletonDeps.Name}}Deps{{end -}}
	  {{- if .ScopedDeps}}, scopedDeps: {{.Name}}Deps{{end}}) =>
		{{.OutputType.ImportAlias}}.{{.OutputType.Name}};
export const provide{{.Name}}: Provide{{.Name}} = impl.provide{{.Name}};
{{- end}}
{{- end}}
{{end}}` +

			// Server template
			`{{define "Server"}}// This file was automatically generated.

import "source-map-support/register"
import { Server, ServerCredentials } from "@grpc/grpc-js";
import yargs from "yargs/yargs";
import { DependencyGraph } from "foundation-runtime";

{{- template "Imports" . -}}

interface Deps {
{{- range $.Services}}
  {{.Name}}: {{.ImportAlias}}.` + singletonNameBase + `Deps;
{{- end}}
}

const prepareDeps = (dg: DependencyGraph): Deps => ({
{{- range $.Services}}
	{{.Name}}: {{.ImportAlias}}.make` + singletonNameBase + `Deps(dg),
{{- end}}
});

const wireServices = (server: Server, deps: Deps): void => {
{{- range $.Services}}
  {{.ImportAlias}}.wireService(deps.{{.Name}}, server);
{{- end}}
};

const argv = yargs(process.argv.slice(2))
		.options({
			listen_hostname: { type: "string" },
			port: { type: "number" },
		})
		.parse();

const server = new Server();
const dg = new DependencyGraph();
wireServices(server, prepareDeps(dg));

console.log(` + "`" + `Starting the server on ${argv.listen_hostname}:${argv.port}` + "`" + `);

server.bindAsync(` + "`" + `${argv.listen_hostname}:${argv.port}` + "`" + `, ServerCredentials.createInsecure(), () => {
  server.start();
  console.log(` + "`" + `Server started.` + "`" + `);
});
{{end}}` +

			// Node stub template
			`{{define "Node stub"}}import { Server } from "@grpc/grpc-js";
import { Deps, WireService } from "./deps.fn";
import { {{.ServiceServerName}}, {{.ServiceName}} } from "./{{.ServiceFileName}}_grpc_pb";

export const wireService: WireService = (_: Deps, server: Server): void => {
const service: {{.ServiceServerName}} = {
	// TODO: implement
};

server.addService({{.ServiceName}}, service);
};{{end}}`))
)

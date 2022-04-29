// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type nodeTmplOptions struct {
	Imports   []tmplSingleImport
	Service   *tmplService
	Package   tmplPackage
	Providers []tmplProvider
}
type serverTmplOptions struct {
	Imports  []tmplSingleImport
	Services []tmplImportedType
}

type nodeImplTmplOptions struct {
	ServiceServerName, ServiceName, ServiceFileName string
}

type tmplService struct {
	GrpcServerImportAlias string
}

type tmplPackage struct {
	Name string
	// nil if the package has no dependencis.
	Deps *tmplDeps
}

type tmplProvider struct {
	Name       string
	InputType  tmplImportedType
	OutputType tmplImportedType
	// nil if the provider has no dependencis.
	Deps            *tmplDeps
	PackageDepsName *string
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
{{define "DefineDeps" -}}
export interface {{.Name}}Deps {
{{- range .Deps}}
	{{.Name}}: {{.Type.ImportAlias}}.{{.Type.Name}};
{{- end}}
}
{{- end}}	

// Input: tmplDeps
{{define "ConstructDeps" -}}
({
	{{- range .Deps}}
		{{.Name}}: {{.Provider.ImportAlias}}.{{.Provider.Name}}Provider(
			graph,
			{{range .ProviderInput.Comments -}}
			{{if .}}// {{.}}
			{{end}}
			{{- end -}}
			{{.ProviderInputType.ImportAlias}}.{{.ProviderInputType.Name -}}
			.deserializeBinary(Buffer.from("{{.ProviderInput.Base64Content}}", "base64"))),
	{{- end}}
  })
{{- end}}

// Input: tmplPackage
{{define "PackageDef" -}}
{{- if .Deps}}
{{template "DefineDeps" .Deps}}
{{- end}}

export const Package = {
  name: "{{.Name}}",
	
  {{- if .Deps}}
  // Package dependencies are instantiated at most once.
  instantiateDeps: (graph: DependencyGraph) => {{template "ConstructDeps" .Deps}},
	{{- end}}
};
{{- end}}

// Input: tmplProvider
{{define "ProviderDef"}}
{{- if .Deps}}
{{template "DefineDeps" .Deps}}
{{- end}}

export const {{.Name}}Provider = (graph: DependencyGraph, input: {{.InputType.ImportAlias}}.{{.InputType.Name -}}) =>
	provide{{.Name}}(
		input
		{{- if .PackageDepsName}}, 
		graph.instantiatePackageDeps(Package)
		{{- end}}
		{{- if .Deps}},
		// Scoped dependencies that are instantiated for each call to Provide{{.Name}}.
		graph.profileCall(` + "`" + `${Package.name}#{{.Deps.Name}}` + "`" + `, () => {{template "ConstructDeps" .Deps}})
		{{- end}}
  );

export type Provide{{.Name}} = (input: {{.InputType.ImportAlias}}.{{.InputType.Name}}
	  {{- if .PackageDepsName}}, packageDeps: {{.PackageDepsName}}Deps{{end -}}
	  {{- if .Deps}}, deps: {{.Name}}Deps{{end}}) =>
		{{.OutputType.ImportAlias}}.{{.OutputType.Name}};
export const provide{{.Name}}: Provide{{.Name}} = impl.provide{{.Name}};
{{- end}}

{{define "Imports"}}
{{range .Imports -}}
import * as {{.Alias}} from "{{.Package}}"
{{end}}
{{end}}` +

			// Node template
			`{{define "Node"}}{{with $opts := .}}// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph } from "@namespacelabs/foundation";

{{- template "Imports" . -}}

{{- template "PackageDef" .Package}}

{{- if .Service}}

export type WireService = (
	{{- if .Package}}deps: {{.Package.Deps.Name}}Deps, {{end -}}
	server: {{.Service.GrpcServerImportAlias}}.Server) => void;
export const wireService: WireService = impl.wireService;
{{- end}}

{{- range $.Providers}}
{{template "ProviderDef" .}}
{{- end}}
{{- end}}
{{end}}` +

			// Server template
			`{{define "Server"}}// This file was automatically generated.

import { Server, ServerCredentials } from "@grpc/grpc-js";
import { DependencyGraph } from "@namespacelabs/foundation";
import "source-map-support/register"
import yargs from "yargs/yargs";

{{- template "Imports" . -}}

// Returns a list of initialization errors.
const wireServices = (server: Server, graph: DependencyGraph): unknown[] => {
	const errors: unknown[] = [];
{{- range $.Services}}
  try {
		{{.ImportAlias}}.wireService({{.ImportAlias}}.Package.instantiateDeps(graph), server);
	} catch (e) {
		errors.push(e);
	}
{{- end}}
  return errors;
};

const argv = yargs(process.argv.slice(2))
		.options({
			listen_hostname: { type: "string" },
			port: { type: "number" },
		})
		.parse();

const server = new Server();

const graph = new DependencyGraph();
const errors = wireServices(server, graph);
if (errors.length > 0) {
	errors.forEach((e) => console.error(e));
	console.error("%d services failed to initialize.", errors.length)
	process.exit(1);
}

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

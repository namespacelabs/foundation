// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"text/template"
)

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

// Input: tmplPackage
{{define "TransitiveInitializersDef"}}

export const TransitiveInitializers: Initializer[] = [
	{{- if .Initializer}}
	initializer,
	{{- end}}
	{{- range .DepsImportAliases}}
	...{{.}}.TransitiveInitializers,
	{{- end}}
];
{{- end}}

// Input: tmplInitializer
{{define "InitializerDef" -}}
const initializer = {
  package: Package,
	initialize: impl.initialize, 
	
  {{- if .InitializeBefore}}
	before: [
	{{- range .InitializeBefore}}"{{.}}",{{end -}}
	]{{- end}}
	
  {{- if .InitializeAfter}}
	after: [
	{{- range .InitializeAfter}}"{{.}}",{{end -}}
	]{{- end}}
};

export type Prepare = (
	{{- if .PackageDepsName}}deps: {{.PackageDepsName}}Deps{{end -}}) => void;
export const prepare: Prepare = impl.initialize;
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
		graph.instantiateDeps(Package.name, "{{.Deps.Name}}", () => {{template "ConstructDeps" .Deps}})
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
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";

{{- template "Imports" . -}}

{{- template "PackageDef" .Package}}

{{- if .Package.Initializer}}

{{template "InitializerDef" .Package.Initializer }}
{{- end}}

{{- template "TransitiveInitializersDef" .Package}}

{{- if .Service}}

export type WireService = (
	{{- if .Package.Deps}}deps: {{.Package.Deps.Name}}Deps, {{end -}}
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
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import "source-map-support/register"
import yargs from "yargs/yargs";

{{- template "Imports" . -}}

// Returns a list of initialization errors.
const wireServices = (server: Server, graph: DependencyGraph): unknown[] => {
	const errors: unknown[] = [];
{{- range $.Services}}
  try {
		{{.Type.ImportAlias}}.wireService(
			{{- if .HasDeps}}{{.Type.ImportAlias}}.Package.instantiateDeps(graph), {{ end -}}
			server);
	} catch (e) {
		errors.push(e);
	}
{{- end}}
  return errors;
};

const TransitiveInitializers: Initializer[] = [
	{{- range .ImportedInitializersAliases}}
	...{{.}}.TransitiveInitializers,
	{{- end}}
];

const argv = yargs(process.argv.slice(2))
		.options({
			listen_hostname: { type: "string" },
			port: { type: "number" },
		})
		.parse();

const server = new Server();

const graph = new DependencyGraph();
graph.runInitializers(TransitiveInitializers);
const errors = wireServices(server, graph);
if (errors.length > 0) {
	errors.forEach((e) => console.error(e));
	console.error("%d services failed to start.", errors.length)
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
import { ServiceDeps, WireService } from "./deps.fn";
import { {{.ServiceServerName}}, {{.ServiceName}} } from "./{{.ServiceFileName}}_grpc_pb";

export const wireService: WireService = (deps: ServiceDeps, server: Server): void => {
  const service: {{.ServiceServerName}} = {
    // TODO: implement
  };

  server.addService({{.ServiceName}}, service);
};{{end}}`))
)

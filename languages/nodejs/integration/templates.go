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
	{{.Name}}: {{template "Type" .Type}};
{{- end}}
}
{{- end}}

// Input: tmplDeps
{{define "ConstructDeps" -}}
({
	{{- range .Deps}}
		{{.Name}}: {{template "Type" .Provider}}Provider(
			graph
			{{- if not .IsProviderParameterized}},
			{{range .ProviderInput.Comments -}}
			{{if .}}// {{.}}
			{{end}}
			{{- end -}}
			{{template "Type" .ProviderInputType -}}
			.deserializeBinary(Buffer.from("{{.ProviderInput.Base64Content}}", "base64"))
			{{- end -}}
		  {{- if .IsProviderParameterized}},
			{{template "Type" .Type}}{{end}}),
	{{- end}}
  })
{{- end}}

// Input: tmplPackage
{{define "PackageDef" -}}
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
	{{- range .ImportedInitializers}}
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
{{- end}}

// Input: tmplProvider
{{define "ProviderInternalDef"}}
export const {{.Name}}Provider = {{if .IsParameterized}}<T>{{end -}}
  (graph: DependencyGraph
	{{- if not .IsParameterized }}, input: {{template "Type" .InputType -}}{{end}}
  {{- if .IsParameterized}}, outputTypeCtr: new (...args: any[]) => T{{end}}) =>
	api.provide{{.Name}}(
		{{if not .IsParameterized}}input{{end}}
		{{- if .PackageDepsName}},
		graph.instantiatePackageDeps(Package)
		{{- end}}
		{{- if .Deps}},
		// Scoped dependencies that are instantiated for each call to Provide{{.Name}}.
		graph.instantiateDeps(Package.name, "{{.Deps.Name}}", () => {{template "ConstructDeps" .Deps}})
		{{- end}}
		{{- if .IsParameterized}}outputTypeCtr{{end}}
  );
{{- end}}

// Input: tmplProvider
{{define "ProviderApiDef"}}
{{- if .Deps -}}
{{template "DefineDeps" .Deps}}

{{end -}}

export type Provide{{.Name}} = {{if .IsParameterized}}<T>{{end -}}
    ({{- if not .IsParameterized}}input: {{template "Type" .InputType}}{{end}}
	  {{- if .PackageDepsName}}, packageDeps: {{.PackageDepsName}}Deps{{end -}}
	  {{- if .Deps}}, deps: {{.Name}}Deps{{end}}
		{{- if .IsParameterized}}outputTypeCtr: new (...args: any[]) => T{{end}}) =>
		{{if .IsParameterized}}T{{else}}{{template "Type" .OutputType}}{{end}};
export const provide{{.Name}}: Provide{{.Name}} = impl.provide{{.Name}};
{{- end}}

{{define "Imports"}}
{{range .Imports -}}
import * as {{.Alias}} from "{{.Package}}"
{{end}}
{{end}}

// Type: tmplImportedType
{{define "Type" -}}
{{if .ImportAlias}}{{.ImportAlias}}.{{end}}{{.Name}}
{{- end}}` +

			// Node "internal" template
			`{{define "NodeInternal"}}{{with $opts := .}}// This file was automatically generated.
// Contains Foundation-internal wiring, the user doesn't interact directly with it.

import * as impl from "./impl";
import * as api from "./api.fn";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";

{{- template "Imports" . -}}

{{- template "PackageDef" .Package}}

{{- if .Package.Initializer}}

{{template "InitializerDef" .Package.Initializer }}
{{- end}}

{{- template "TransitiveInitializersDef" .Package}}

{{- range $.Providers}}
{{template "ProviderInternalDef" .}}
{{- end}}
{{- end}}
{{end}}` +

			// Node "api" template
			`{{define "NodeApi"}}{{with $opts := .}}// This file was automatically generated.
// Contains type and function definitions that needs to be implemented in "impl.ts".

import * as impl from "./impl";
import { Registrar } from "@namespacelabs/foundation";

{{- template "Imports" . -}}

{{if .Package.Deps -}}
{{template "DefineDeps" .Package.Deps}}

{{end}}

{{- if .Package.Initializer -}}
export type Prepare = (
	{{- if .Package.Initializer.PackageDepsName}}deps: {{.Package.Initializer.PackageDepsName}}Deps{{end -}}) => void;
export const prepare: Prepare = impl.initialize;

{{end}}

{{- if .Service -}}
export type WireService = (
	{{- if .Package.Deps}}deps: {{.Package.Deps.Name}}Deps, {{end -}}
	registrar: Registrar) => void;
export const wireService: WireService = impl.wireService;
{{- end}}

{{- range $.Providers -}}
{{template "ProviderApiDef" .}}
{{- end}}
{{- end}}
{{end}}` +

			// Server template
			`{{define "Server"}}// This file was automatically generated.

import { DependencyGraph, Initializer, Server } from "@namespacelabs/foundation";

{{- template "Imports" . -}}

// Returns a list of initialization errors.
const wireServices = (server: Server, graph: DependencyGraph): unknown[] => {
	const errors: unknown[] = [];
{{- range $.Services}}
  try {
		{{.Type.ImportAlias}}.wireService(
			{{- if .HasDeps}}{{.InternalImportAlias}}.Package.instantiateDeps(graph), {{ end -}}
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

const server = new Server();

const graph = new DependencyGraph();
graph.runInitializers(TransitiveInitializers);
const errors = wireServices(server, graph);
if (errors.length > 0) {
	errors.forEach((e) => console.error(e));
	console.error("%d services failed to start.", errors.length)
	process.exit(1);
}

server.start();
{{end}}` +

			// Node stub template
			`{{define "Node stub"}}import { Registrar } from "@namespacelabs/foundation";
import { ServiceDeps, WireService } from "./api.fn";
import { {{.ServiceServerName}}, {{.ServiceName}} } from "./{{.ServiceFileName}}_grpc_pb";

export const wireService: WireService = (deps: ServiceDeps, registrar: Registrar): void => {
  const service: {{.ServiceServerName}} = {
    // TODO: implement
  };

  registrar.registerGrpcService({{.ServiceName}}, service);
};{{end}}`))
)

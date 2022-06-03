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

export type Prepare = (
	{{- if .PackageDepsName}}deps: {{.PackageDepsName}}Deps{{end -}}) => Promise<void> | void;
export const prepare: Prepare = impl.initialize;
{{- end}}

// Input: tmplProvider
{{define "ProviderDef"}}
{{- if .Deps}}
{{template "DefineDeps" .Deps}}
{{- end}}

export const {{.Name}}Provider = {{if .IsParameterized}}<T>{{end -}}
	(graph: DependencyGraph
	{{- if not .IsParameterized }}, input: {{template "Type" .InputType -}}{{end}}
	{{- if .IsParameterized}}, outputTypeCtr: new (...args: any[]) => T{{end}}) =>
	provide{{.Name}}(
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
import * as {{.Alias}} from "{{.Package}}";
{{end}}
{{end}}

// Type: tmplImportedType
{{define "Type" -}}
	{{if .ImportAlias}}{{.ImportAlias}}.{{end}}{{.Name}}
	{{- if .Parameters}}<
		{{- range $index, $p := .Parameters}}
			{{- if ne $index 0 }}, {{end}}
			{{- template "Type" $p}}
		{{- end -}}
	>{{- end}}
{{- end}}` +

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
	{{- if .Package.Deps}}deps: {{.Package.Deps.Name}}Deps{{end -}}) => Promise<void>;
export const wireService: WireService = impl.wireService;
{{- end}}

{{- range $.Providers}}
{{template "ProviderDef" .}}
{{- end}}
{{- end}}
{{end}}` +

			// Server template
			`{{define "Server"}}// This file was automatically generated.

import { DependencyGraph, Initializer } from "@namespacelabs/foundation";

{{- template "Imports" . -}}
import {provideGrpcRegistrar, GrpcServer} from "@namespacelabs.dev-foundation/std-nodejs-grpc/impl"
import {provideHttpServer, HttpServerImpl} from "@namespacelabs.dev-foundation/std-nodejs-http/impl"

// Returns a list of initialization errors.
const wireServices = async (graph: DependencyGraph): Promise<unknown[]> => {
	const errors: unknown[] = [];
{{- range $.Services}}
	try {
		await {{.Type.ImportAlias}}.wireService(
			{{- if .HasDeps}}{{.Type.ImportAlias}}.Package.instantiateDeps(graph){{ end -}});
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

async function main() {
	const graph = new DependencyGraph();
	await graph.runInitializers(TransitiveInitializers);
	const errors = await wireServices(graph);
	if (errors.length > 0) {
		errors.forEach((e) => console.error(e));
		console.error("%d services failed to start.", errors.length);
		process.exit(1);
	}

	(provideGrpcRegistrar() as GrpcServer).start();
	((await provideHttpServer()) as HttpServerImpl).start();
}

main();
{{end}}` +

			// Node stub template
			`{{define "Node stub" -}}
import { ServiceDeps, WireService } from "./deps.fn";
import { {{.ServiceServerName}}, {{.ServiceName}} } from "./{{.ServiceFileName}}_grpc_pb";

export const wireService: WireService = async (deps: ServiceDeps) => {
	const service: {{.ServiceServerName}} = {
		// TODO: implement
	};

	deps.grpc.registerGrpcService({{.ServiceName}}, service);
};{{end}}`))
)

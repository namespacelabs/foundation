// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type nodeTmplOptions struct {
	Imports       []tmplSingleImport
	Service       *tmplService
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

type tmplService struct {
	GrpcServerImportAlias string
}

type tmplProvider struct {
	Name       string
	InputType  tmplImportedType
	OutputType tmplImportedType
	ScopedDeps *tmplDeps
}

type tmplDeps struct {
	Name string
	// Key for this dependency list, globally unique.
	Key                 string
	DepGraphImportAlias string
	Deps                []tmplDependency
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
{{define "DepsFactory"}}
export interface {{.Name}}Deps {
{{- range .Deps}}
	{{.Name}}: {{.Type.ImportAlias}}.{{.Type.Name}};
{{- end}}
}

export const {{.Name}}DepsFactory = {
	key: "{{.Key}}",
  instantiate: (dg: {{.DepGraphImportAlias}}.DependencyGraph): {{.Name}}Deps => ({
		{{- range .Deps}}
			{{- range .ProviderInput.Comments}}
			// {{.}}
			{{- end}}
			{{.Name}}: dg.instantiate({
				{{- if .HasSingletonDeps}}
				singletonDepsFactory: {{.Provider.ImportAlias}}.` + singletonNameBase + `DepsFactory,
				{{- end}}
				{{- if .HasScopedDeps}}
				scopedDepsFactory: {{.Provider.ImportAlias}}.{{.Provider.Name}}DepsFactory,
				{{- end}}
				providerFn: (params) =>			
					{{.Provider.ImportAlias}}.provide{{.Provider.Name}}(
						{{.ProviderInputType.ImportAlias}}.{{.ProviderInputType.Name -}}
						.deserializeBinary(Buffer.from("{{.ProviderInput.Base64Content}}", "base64"))
					{{- if .HasSingletonDeps}},
					  params.singletonDeps!
					{{- end}}
					{{- if .HasScopedDeps}},
					  params.scopedDeps!
					{{- end}})
			}),
		{{- end}}
	})
};
{{- end}}

{{define "Imports"}}
{{range .Imports -}}
import * as {{.Alias}} from "{{.Package}}"
{{end}}
{{end}}` +

			// Node template
			`{{define "Node"}}{{with $opts := .}}// This file was automatically generated.

import * as impl from "./impl";

{{- template "Imports" . -}}

{{if .SingletonDeps}}
// Singleton dependencies are instantiated at most once.
{{- template "DepsFactory" .SingletonDeps}}
{{- end}}

{{- if .Service}}

export type WireService = (
	{{- if .SingletonDeps}}deps: {{.SingletonDeps.Name}}Deps, {{end -}}
	server: {{.Service.GrpcServerImportAlias}}.Server) => void;
export const wireService: WireService = impl.wireService;
{{- end}}

{{- range $.Providers -}}

{{if .ScopedDeps}}
// Scoped dependencies that are instantiated for each call to Provide{{.Name}}.
{{template "DepsFactory" .ScopedDeps}}
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

import { Server, ServerCredentials } from "@grpc/grpc-js";
import { DependencyGraph } from "foundation-runtime";
import "source-map-support/register"
import yargs from "yargs/yargs";

{{- template "Imports" . -}}

const wireServices = (server: Server, dg: DependencyGraph): void => {
{{- range $.Services}}
  dg.instantiate({
    singletonDepsFactory: {{.ImportAlias}}.` + singletonNameBase + `DepsFactory,
    providerFn: (params) => {{.ImportAlias}}.wireService(params.singletonDeps!, server),
  })
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
wireServices(server, dg);

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

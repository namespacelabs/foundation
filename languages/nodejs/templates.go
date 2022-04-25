// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type nodeTmplOptions struct {
	Imports   []tmplSingleImport
	Service   *tmplDeps
	Providers []tmplProvider
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
{{define "Deps"}}
export interface {{.Name}}Deps {
{{range .Deps}}
	{{.Name}}: {{.Type.ImportAlias}}.{{.Type.Name}};{{end}}
}

export const make{{.Name}}Deps = (): {{.Name}}Deps => ({
	{{range .Deps}}
	  {{range .ProviderInput.Comments}}
		// {{.}}{{end}}
		{{.Name}}: {{.Provider.ImportAlias}}.provide{{.Provider.Name}}(
			{{.ProviderInputType.ImportAlias}}.{{.ProviderInputType.Name}}.deserializeBinary(
				Buffer.from("{{.ProviderInput.Base64Content}}", "base64"))),{{end}}
});
{{end}}` +

			// Node template
			`{{define "Node"}}// This file was automatically generated.

{{if .Service}}
import { Server } from "@grpc/grpc-js";
{{end}}
import * as impl from "./impl";

{{range .Imports}}
import * as {{.Alias}} from "{{.Package}}"{{end}}

{{if .Service}}
{{template "Deps" .Service}}

export type WireService = (deps: {{.Service.Name}}Deps, server: Server) => void;
export const wireService: WireService = impl.wireService;
{{end}}

{{range $.Providers}}

{{if .ScopedDeps}}
// Scoped dependencies that are instantiated for each call to Provide{{.Name}}.
{{template "Deps" .ScopedDeps}}
{{end}}

export type Provide{{.Name}} = (input: {{.InputType.ImportAlias}}.{{.InputType.Name}}
	  {{if .ScopedDeps}}, deps: {{.Name}}Deps{{end}}) =>
		{{.OutputType.ImportAlias}}.{{.OutputType.Name}};
export const provide{{.Name}}: Provide{{.Name}} = impl.provide{{.Name}};
{{end}}{{end}}` +

			// Server template
			`{{define "Server"}}// This file was automatically generated.

import 'source-map-support/register'
import { Server, ServerCredentials } from "@grpc/grpc-js";
import yargs from "yargs/yargs";

{{range .Imports}}
import * as {{.Alias}} from "{{.Package}}"{{end}}

interface Deps {
{{range $.Services}}
{{.Name}}: {{.ImportAlias}}.ServiceDeps;{{end}}
}

const prepareDeps = (): Deps => ({
	{{range $.Services}}
	{{.Name}}: {{.ImportAlias}}.makeServiceDeps(),{{end}}
});

const wireServices = (server: Server, deps: Deps): void => {
{{range $.Services}}
{{.ImportAlias}}.wireService(deps.{{.Name}}!, server);{{end}}
};

const argv = yargs(process.argv.slice(2))
.options({
	listen_hostname: { type: "string" },
	port: { type: "number" },
})
.parse();

const server = new Server();
wireServices(server, prepareDeps());

console.log(` + "`" + `Starting the server on ${argv.listen_hostname}:${argv.port}` + "`" + `);

server.bindAsync(` + "`" + `${argv.listen_hostname}:${argv.port}` + "`" + `, ServerCredentials.createInsecure(), () => {
server.start();

console.log(` + "`" + `Server started.` + "`" + `);
});{{end}}` +

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

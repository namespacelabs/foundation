// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpcgen

import (
	"text/template"
)

var (
	tmpl = template.Must(template.New("template").Parse(
		`// Helper templates

// Input: []SingleImport
{{define "Imports"}}
{{- range .}}
import * as {{.Alias}} from "{{.Package}}";
{{- end -}}
{{end}}

// Input: tmplImportedType
{{define "Type" -}}
{{if .ImportAlias}}{{.ImportAlias}}.{{end}}{{.Name}}
{{- end}}

// Input: tmplProtoService
{{define "ServiceDef" -}}
// {{.Name}}

const {{.Name}}Definition: grpc.ServiceDefinition = {
	{{- range .Methods}}
	{{.Name}}: {
		path: "{{.Path}}",
		originalName: "{{.OriginalName}}",
		requestStream: false,
		responseStream: false,
		requestSerialize: (arg) => Buffer.from(arg.serializeBinary()),
		requestDeserialize: (arg) => {{template "Type" .RequestType}}.deserializeBinary(new Uint8Array(arg)),
		responseSerialize: (arg) => Buffer.from(arg.serializeBinary()),
		responseDeserialize: (arg) => {{template "Type" .ResponseType}}.deserializeBinary(new Uint8Array(arg)),
	},
  {{- end}}
};
{{- end}}

// Input: tmplProtoService
{{define "ServiceServer" -}}
// {{.Name}} - Server

export interface {{.Name}}Server {
{{- range .Methods}}
	{{.Name}}(request: {{template "Type" .RequestType}}, context: CallContext): Promise<{{template "Type" .ResponseType}}>;
{{- end}}
}

export function bind{{.Name}}Server(server: {{.Name}}Server) {
	return {
		impl: server,
		definition: {{.Name}}Definition,
	}
}
{{- end}}

// Input: tmplProtoService
{{define "ServiceClient" -}}
// {{.Name}} - Client

export interface {{.Name}}Client {
{{- range .Methods}}
	{{.Name}}(request: {{template "Type" .RequestType}}, options?: CallOptions): Promise<{{template "Type" .ResponseType}}>;
{{- end}}
}

export function new{{.Name}}Client(address: string, credentials: grpc.ChannelCredentials, options?: object): {{.Name}}Client {
	return adaptClient(wrapped{{.Name}}ClientConstructor, address, credentials, options) as {{.Name}}Client;
}
{{- end}}

// Input: tmplProtoService
{{define "ServiceClientWiring" -}}
const wrapped{{.Name}}ClientConstructor = grpc.makeGenericClientConstructor({{.Name}}Definition, "Unused service name");
{{- end}}
` +
			`{{define "File"}}// This file was automatically generated.

import * as grpc from "@grpc/grpc-js";

{{- template "Imports" .Imports}}
{{- if .Opts.GenClients}}
import {adaptClient, CallOptions} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/client";
{{- end}}
{{- if .Opts.GenServers}}
import {CallContext} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/server";
{{- end}}

// API

{{- if .Opts.GenClients}}
{{- range .Services}}

{{template "ServiceClient" . }}
{{- end -}}
{{- end}}

{{- if .Opts.GenServers}}
{{- range .Services}}

{{template "ServiceServer" . }}
{{- end -}}
{{- end}}

// Wiring

{{- range .Services}}

{{template "ServiceDef" . }}
{{- end -}}

{{- if .Opts.GenClients}}
{{- range .Services}}

{{template "ServiceClientWiring" . }}
{{- end -}}
{{- end}}

{{end}}`))
)

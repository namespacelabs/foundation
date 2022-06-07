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
{{define "Service" -}}
// {{.Name}}

export interface {{.Name}}Server {
{{- range .Methods}}
	{{.Name}}: (request: {{template "Type" .RequestType}}) => Promise<{{template "Type" .ResponseType}}>;
{{- end}}
}

export class {{.Name}}Client {
	readonly #wrappedClient: { [methodName: string]: Function };

	constructor(address: string, credentials: grpc.ChannelCredentials, options?: object) {
		this.#wrappedClient = new wrapped{{.Name}}ClientConstructor(address, credentials, options);
	}
{{- range .Methods}}

	async {{.Name}}(request: {{template "Type" .RequestType}}): Promise<{{template "Type" .ResponseType}}> {
		const method = this.#wrappedClient.{{.Name}}.bind(this.#wrappedClient);
		return new Promise<{{template "Type" .ResponseType}}>((resolve, reject) => {
			method(
				request,
				(err: any, response: any) => {
					if (err != null) {
						reject(err);
					} else {
						resolve(response);
					}
				},
			);
		});
	}
{{- end}}
}


const wrapped{{.Name}}ClientConstructor = grpc.makeGenericClientConstructor({
	{{- range .Methods}}

	{{.Name}}: {
		path: "{{.Path}}",
		requestStream: false,
		responseStream: false,
		requestSerialize: (arg) => Buffer.from(arg.serializeBinary()),
		requestDeserialize: (arg) => {{template "Type" .RequestType}}.deserializeBinary(new Uint8Array(arg)),
		responseSerialize: (arg) => Buffer.from(arg.serializeBinary()),
		responseDeserialize: (arg) => {{template "Type" .ResponseType}}.deserializeBinary(new Uint8Array(arg)),
	},
  {{- end}}
}, "Unused service name");
{{- end}}
` +
			`{{define "File"}}// This file was automatically generated.

import * as grpc from "@grpc/grpc-js";

{{- template "Imports" .Imports -}}

{{range .Services}}

{{template "Service" . }}
{{- end -}}

{{end}}`))
)

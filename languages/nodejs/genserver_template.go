// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type serverTmplOptions struct {
	Imports  []singleImport
	Services []service
}

type service struct {
	ImportAlias, Name string
}

type singleImport struct {
	Alias, Package string
}

var (
	serverTmpl = template.Must(template.New("template").Parse(
		`// This file was automatically generated.
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// XXX This file is generated.

import { Server, ServerCredentials } from "@grpc/grpc-js";
import yargs from "yargs/yargs";
{{range .Imports}}
import * as {{.Alias}} from "{{.Package}}"{{end}}

interface Deps {
{{range $.Services}}
	{{.Name}}: {{.ImportAlias}}.Deps;{{end}}
}

const prepareDeps = (): Deps => ({
{{range $.Services}}
	{{.Name}}: {}{{end}}
});

const wireServices = (server: Server, deps: Deps): void => {
{{range $.Services}}
	{{.ImportAlias}}.wireService(deps.{{.Name}}, server);{{end}}
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
});`))
)

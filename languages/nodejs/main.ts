// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// XXX This file is generated.

import { Server, ServerCredentials } from "@grpc/grpc-js";
import yargs from "yargs/yargs";
import { prepareDeps, wireServices } from "./deps";

const argv = yargs(process.argv.slice(2))
	.options({
		listen_hostname: { type: "string" },
		port: { type: "number" },
	})
	.parse();

const server = new Server();
wireServices(server, prepareDeps());

console.log(`Starting the server on ${argv.listen_hostname}:${argv.port}`);

server.bindAsync(`${argv.listen_hostname}:${argv.port}`, ServerCredentials.createInsecure(), () => {
	server.start();

	console.log(`Server started.`);
});

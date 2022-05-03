// This file was automatically generated.

import { Server, ServerCredentials } from "@grpc/grpc-js";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import "source-map-support/register"
import yargs from "yargs/yargs";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/deps.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-numberformatter/deps.fn"
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/deps.fn"

// Returns a list of initialization errors.
const wireServices = (server: Server, graph: DependencyGraph): unknown[] => {
	const errors: unknown[] = [];
  try {
		i0.wireService(server);
	} catch (e) {
		errors.push(e);
	}
  try {
		i1.wireService(i1.Package.instantiateDeps(graph), server);
	} catch (e) {
		errors.push(e);
	}
  return errors;
};

const TransitiveInitializers: Initializer[] = [
	...i2.TransitiveInitializers,
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

console.log(`Starting the server on ${argv.listen_hostname}:${argv.port}`);

server.bindAsync(`${argv.listen_hostname}:${argv.port}`, ServerCredentials.createInsecure(), () => {
  server.start();
  console.log(`Server started.`);
});

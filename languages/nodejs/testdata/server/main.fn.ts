// This file was automatically generated.

import { DependencyGraph, Initializer, Server } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/deps.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-numberformatter/deps.fn"
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-postuser/deps.fn"

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
  try {
		i2.wireService(i2.Package.instantiateDeps(graph), server);
	} catch (e) {
		errors.push(e);
	}
  return errors;
};

const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
	...i1.TransitiveInitializers,
	...i2.TransitiveInitializers,
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

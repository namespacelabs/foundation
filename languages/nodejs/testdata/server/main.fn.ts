// This file was automatically generated.

import { DependencyGraph, Initializer, Server } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/api.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/internal.fn"
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simplehttp/api.fn"
import * as i3 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simplehttp/internal.fn"
import * as i4 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-numberformatter/api.fn"
import * as i5 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-numberformatter/internal.fn"
import * as i6 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-postuser/api.fn"
import * as i7 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-postuser/internal.fn"

// Returns a list of initialization errors.
const wireServices = (server: Server, graph: DependencyGraph): unknown[] => {
	const errors: unknown[] = [];
  try {
		i0.wireService(server);
	} catch (e) {
		errors.push(e);
	}
  try {
		i2.wireService(server);
	} catch (e) {
		errors.push(e);
	}
  try {
		i4.wireService(i5.Package.instantiateDeps(graph), server);
	} catch (e) {
		errors.push(e);
	}
  try {
		i6.wireService(i7.Package.instantiateDeps(graph), server);
	} catch (e) {
		errors.push(e);
	}
  return errors;
};

const TransitiveInitializers: Initializer[] = [
	...i1.TransitiveInitializers,
	...i3.TransitiveInitializers,
	...i5.TransitiveInitializers,
	...i7.TransitiveInitializers,
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

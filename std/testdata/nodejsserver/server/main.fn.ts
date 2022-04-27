// This file was automatically generated.

import { Server, ServerCredentials } from "@grpc/grpc-js";
import { DependencyGraph } from "@namespacelabs/foundation";
import "source-map-support/register"
import yargs from "yargs/yargs";

// Returns a list of initialization errors.
const wireServices = (server: Server, dg: DependencyGraph): unknown[] => {
	const errors: unknown[] = [];
  return errors;
};

const argv = yargs(process.argv.slice(2))
		.options({
			listen_hostname: { type: "string" },
			port: { type: "number" },
		})
		.parse();

const server = new Server();

const dg = new DependencyGraph();
const errors = wireServices(server, dg);
if (errors.length > 0) {
	errors.forEach((e) => console.error(e));
	console.error("%d services failed to initialize.", errors.length)
	process.exit(1);
}

console.log(`Starting the server on ${argv.listen_hostname}:${argv.port}`);

server.bindAsync(`${argv.listen_hostname}:${argv.port}`, ServerCredentials.createInsecure(), () => {
  server.start();
  console.log(`Server started.`);
});

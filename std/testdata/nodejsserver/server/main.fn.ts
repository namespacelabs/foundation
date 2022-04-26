// This file was automatically generated.

import { Server, ServerCredentials } from "@grpc/grpc-js";
import { DependencyGraph } from "foundation-runtime";
import "source-map-support/register"
import yargs from "yargs/yargs";

<<<<<<< HEAD


interface Deps {

}

const prepareDeps = (): Deps => ({
	
});

const wireServices = (server: Server, deps: Deps): void => {

};

const argv = yargs(process.argv.slice(2))
.options({
	listen_hostname: { type: "string" },
	port: { type: "number" },
})
.parse();
=======
const wireServices = (server: Server, dg: DependencyGraph): void => {
};

const argv = yargs(process.argv.slice(2))
		.options({
			listen_hostname: { type: "string" },
			port: { type: "number" },
		})
		.parse();
>>>>>>> Tidy/generate.

const server = new Server();
const dg = new DependencyGraph();
wireServices(server, dg);

console.log(`Starting the server on ${argv.listen_hostname}:${argv.port}`);

server.bindAsync(`${argv.listen_hostname}:${argv.port}`, ServerCredentials.createInsecure(), () => {
<<<<<<< HEAD
server.start();

console.log(`Server started.`);
});
=======
  server.start();
  console.log(`Server started.`);
});
>>>>>>> Tidy/generate.

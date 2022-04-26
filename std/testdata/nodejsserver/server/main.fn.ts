// This file was automatically generated.

import 'source-map-support/register'
import { Server, ServerCredentials } from "@grpc/grpc-js";
import yargs from "yargs/yargs";



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

const server = new Server();
wireServices(server, prepareDeps());

console.log(`Starting the server on ${argv.listen_hostname}:${argv.port}`);

server.bindAsync(`${argv.listen_hostname}:${argv.port}`, ServerCredentials.createInsecure(), () => {
server.start();

console.log(`Server started.`);
});
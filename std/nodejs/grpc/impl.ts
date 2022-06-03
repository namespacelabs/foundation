// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import * as grpc from "@grpc/grpc-js";
import yargs from "yargs/yargs";
import { GrpcRegistrar } from "./registrar";

const argv = yargs(process.argv.slice(2))
	.options({
		listen_hostname: { type: "string" },
		port: { type: "number" },
	})
	.parse();

export class GrpcServer implements GrpcRegistrar {
	readonly #server = new grpc.Server();

	registerGrpcService(
		service: grpc.ServiceDefinition,
		implementation: grpc.UntypedServiceImplementation
	): void {
		this.#server.addService(service, implementation);
	}

	async start() {
		if (!argv.port) {
			return;
		}

		console.log(`Starting the gRPC server on ${argv.listen_hostname}:${argv.port}`);

		this.#server.bindAsync(
			`${argv.listen_hostname}:${argv.port}`,
			grpc.ServerCredentials.createInsecure(),
			() => {
				this.#server.start();
				console.log(`gRPC server started.`);
			}
		);
	}
}

let grpcServer: GrpcServer | undefined;

export const provideGrpcRegistrar = () => {
	if (!grpcServer) {
		grpcServer = new GrpcServer();
	}
	return grpcServer;
};

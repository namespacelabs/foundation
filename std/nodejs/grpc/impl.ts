// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { grpc } from "@namespacelabs.dev-foundation/std-nodejs-grpcgen";
import { adaptServer, BoundService } from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/server";
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

	registerGrpcService<T>(serviceDef: BoundService<T>): void {
		this.#server.addService(...adaptServer(serviceDef));
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

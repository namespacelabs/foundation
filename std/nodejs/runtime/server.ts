// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import * as grpc from "@grpc/grpc-js";
import "source-map-support/register";
import yargs from "yargs/yargs";
import { Registrar } from "./registrar";

const argv = yargs(process.argv.slice(2))
	.options({
		listen_hostname: { type: "string" },
		port: { type: "number" },
		http_port: { type: "number" },
	})
	.parse();

export class Server implements Registrar {
	readonly #grpcServer = new grpc.Server();

	registerGrpcService(
		service: grpc.ServiceDefinition,
		implementation: grpc.UntypedServiceImplementation
	): void {
		this.#grpcServer.addService(service, implementation);
	}

	async #startGrpcServer() {
		if (!argv.port) {
			return;
		}

		console.log(`Starting the gRPC server on ${argv.listen_hostname}:${argv.port}`);

		this.#grpcServer.bindAsync(
			`${argv.listen_hostname}:${argv.port}`,
			grpc.ServerCredentials.createInsecure(),
			() => {
				this.#grpcServer.start();
				console.log(`gRPC server started.`);
			}
		);
	}

	async start(): Promise<void> {
		await this.#startGrpcServer();
	}
}

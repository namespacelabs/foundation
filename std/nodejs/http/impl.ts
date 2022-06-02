// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import fastify from "fastify";
import middie from "middie";
import yargs from "yargs";
import { HttpServer } from "./httpserver";

const argv = yargs(process.argv.slice(2))
	.options({
		http_port: { type: "number" },
	})
	.parse();

export class HttpServerImpl implements HttpServer {
	readonly #fastifyServer = fastify({
		logger: true,
	});

	async init() {
		await this.#fastifyServer.register(middie);
	}

	fastify() {
		return this.#fastifyServer;
	}

	async start() {
		if (!argv.http_port) {
			return;
		}

		console.log(`Starting the HTTP server on ${argv.listen_hostname}:${argv.http_port}`);

		this.#fastifyServer.listen(argv.http_port!, (err) => {
			if (err) {
				this.#fastifyServer.log.error(err);
				process.exit(1);
			}

			console.log(`HTTP server started.`);
		});
	}
}

let httpServer: Promise<HttpServerImpl> | undefined;

export const provideHttpServer = (): Promise<HttpServer> => {
	if (!httpServer) {
		httpServer = (async () => {
			console.log("Initializing the HTTP server.");
			const server = new HttpServerImpl();
			await server.init();
			return server;
		})();
	}
	return httpServer;
};

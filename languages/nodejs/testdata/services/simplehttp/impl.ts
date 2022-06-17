// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import type { OpenTelemetryReqInstance } from "@autotelic/fastify-opentelemetry";
import { ServiceDeps, WireService } from "./deps.fn";

// TODO: find out why the "fastify" module augmentation from "@autotelic/fastify-opentelemetry" doesn't work.
declare module "fastify" {
	interface FastifyRequest {
		readonly openTelemetry: () => OpenTelemetryReqInstance;
	}
}

export const wireService: WireService = async (deps: ServiceDeps): Promise<void> => {
	const server = await deps.httpServer;

	server.fastify().post("/simple/:userId", async (req) => {
		const { tracer } = req.openTelemetry();
		tracer.startSpan(`test manual span`).end();
		const params = req.params as any;
		return { output: `Hello world! User ID: ${params["userId"]}` };
	});
};

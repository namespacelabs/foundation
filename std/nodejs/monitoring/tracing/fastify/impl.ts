// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import openTelemetryPlugin from "@autotelic/fastify-opentelemetry";
import { ExtensionDeps } from "./deps.fn";

export const initialize = async (deps: ExtensionDeps) => {
	const server = await deps.httpServer;
	server.fastify().register(openTelemetryPlugin, { wrapRoutes: true });

	console.log(`Initialized OpenTelemetry instrumentation for the HTTP server.`);
};

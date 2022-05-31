// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { ServiceDeps, WireService } from "./deps.fn";

export const wireService: WireService = async (deps: ServiceDeps): Promise<void> => {
	const server = await deps.httpServer;

	server.fastify().post("/simple/:userId", async (req) => {
		const params = req.params as any;
		return { output: `Hello world! User ID: ${params["userId"]}` };
	});
};

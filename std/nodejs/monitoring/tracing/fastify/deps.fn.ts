// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer, Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-nodejs-http/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/std-nodejs-http/provider_pb";
import * as i2 from "@namespacelabs.dev-foundation/std-nodejs-http/httpserver";


export interface ExtensionDeps {
	httpServer: Promise<i2.HttpServer>;
}

export const Package = {
	name: "namespacelabs.dev/foundation/std/nodejs/monitoring/tracing/fastify",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		httpServer: i0.HttpServerProvider(
			graph,
			i1.NoArgs.deserializeBinary(Buffer.from("", "base64"))),
	}),
};

const initializer = {
	package: Package,
	initialize: impl.initialize,
};

export type Prepare = (deps: ExtensionDeps) => Promise<void> | void;
export const prepare: Prepare = impl.initialize;

export const TransitiveInitializers: Initializer[] = [
	initializer,
	...i0.TransitiveInitializers,
];

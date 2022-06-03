// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-nodejs-monitoring-tracing/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/std-nodejs-monitoring-tracing/types_pb";
import * as i2 from "@namespacelabs.dev-foundation/std-nodejs-monitoring-tracing/exporter";


export interface ExtensionDeps {
	openTelemetry: i2.Exporter;
}

export const Package = {
	name: "namespacelabs.dev/foundation/std/nodejs/monitoring/tracing/jaeger",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		openTelemetry: i0.ExporterProvider(
			graph,
			// name: "jaeger"
			i1.ExporterArgs.deserializeBinary(Buffer.from("CgZqYWVnZXI=", "base64"))),
	}),
};

const initializer = {
	package: Package,
	initialize: impl.initialize,
	before: ["namespacelabs.dev/foundation/std/nodejs/monitoring/tracing",]
};

export type Prepare = (deps: ExtensionDeps) => Promise<void> | void;
export const prepare: Prepare = impl.initialize;

export const TransitiveInitializers: Initializer[] = [
	initializer,
	...i0.TransitiveInitializers,
];

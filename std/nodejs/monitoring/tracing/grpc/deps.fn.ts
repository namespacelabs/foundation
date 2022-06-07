// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import {GrpcRegistrar} from "@namespacelabs.dev-foundation/std-nodejs-grpc"
import * as i0 from "@namespacelabs.dev-foundation/std-nodejs-grpc/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/std-nodejs-grpc/provider_pb";
import * as i2 from "@namespacelabs.dev-foundation/std-nodejs-grpc/interceptor";
import * as i3 from "@namespacelabs.dev-foundation/std-nodejs-monitoring-tracing/deps.fn";
import * as i4 from "@namespacelabs.dev-foundation/std-nodejs-monitoring-tracing/types_pb";
import * as i5 from "@namespacelabs.dev-foundation/std-nodejs-monitoring-tracing/api";


export interface ExtensionDeps {
	grpcInterceptorRegistrar: i2.GrpcInterceptorRegistrar;
	tracingRegistrar: i5.InstrumentationRegistrar;
}

export const Package = {
	name: "namespacelabs.dev/foundation/std/nodejs/monitoring/tracing/grpc",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		grpcInterceptorRegistrar: i0.GrpcInterceptorRegistrarProvider(
			graph,
			i1.NoArgs.deserializeBinary(Buffer.from("", "base64"))),
		tracingRegistrar: i3.InstrumentationRegistrarProvider(
			graph,
			i4.NoArgs.deserializeBinary(Buffer.from("", "base64"))),
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
	...i3.TransitiveInitializers,
];

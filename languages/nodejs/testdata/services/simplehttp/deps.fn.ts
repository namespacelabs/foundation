// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs.dev/foundation/std/nodejs/runtime";
import {GrpcRegistrar} from "@namespacelabs.dev/foundation/std/nodejs/grpc"
import * as i0 from "@namespacelabs.dev/foundation/std/nodejs/http/deps.fn";
import * as i1 from "@namespacelabs.dev/foundation/std/nodejs/http/provider_pb";
import * as i2 from "@namespacelabs.dev/foundation/std/nodejs/http/httpserver";


export interface ServiceDeps {
	httpServer: Promise<i2.HttpServer>;
}

export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/simplehttp",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		httpServer: i0.HttpServerProvider(
			graph,
			i1.NoArgs.fromBinary(Buffer.from("", "base64"))),
	}),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps, registrar: GrpcRegistrar) => Promise<void> | void;
export const wireService: WireService = impl.wireService;

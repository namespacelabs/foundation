// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-nodejs-grpc/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/std-nodejs-grpc/provider_pb";
import * as i2 from "@namespacelabs.dev-foundation/std-nodejs-grpc/registrar";


export interface ServiceDeps {
	grpcRegistrar: i2.GrpcRegistrar;
}

export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		grpcRegistrar: i0.GrpcRegistrarProvider(
			graph,
			i1.NoArgs.deserializeBinary(Buffer.from("", "base64"))),
	}),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps) => Promise<void>;
export const wireService: WireService = impl.wireService;

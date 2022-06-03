// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-nodejs-grpc/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/std-nodejs-grpc/provider_pb";
import * as i2 from "@namespacelabs.dev-foundation/std-nodejs-grpc/registrar";
import * as i3 from "@namespacelabs.dev-foundation/std-grpc/deps.fn";
import * as i4 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/service_grpc_pb";


export interface ServiceDeps {
	grpcRegistrar: i2.GrpcRegistrar;
	postService: i4.PostServiceClient;
}

export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/postuser",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		grpcRegistrar: i0.GrpcRegistrarProvider(
			graph,
			i1.NoArgs.deserializeBinary(Buffer.from("", "base64"))),
		postService: i3.BackendProvider(
			graph,
			i4.PostServiceClient),
	}),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
	...i3.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps) => Promise<void>;
export const wireService: WireService = impl.wireService;

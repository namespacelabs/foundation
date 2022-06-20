// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer, InstantiationContext } from "@namespacelabs.dev/foundation/std/nodejs/runtime";
import {GrpcRegistrar} from "@namespacelabs.dev/foundation/std/nodejs/grpc"
import * as i0 from "@namespacelabs.dev/foundation/std/grpc/deps.fn";
import * as i1 from "@namespacelabs.dev/foundation/std/grpc/protos/provider_pb";
import * as i2 from "@namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple/service_grpc.fn";


export interface ServiceDeps {
	postService: i2.PostServiceClient;
}

export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/postuser",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph, context: InstantiationContext) => ({
		postService: i0.BackendProvider(
			graph,
			// package_name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple"
			i1.Backend.fromBinary(Buffer.from("CkZuYW1lc3BhY2VsYWJzLmRldi9mb3VuZGF0aW9uL2xhbmd1YWdlcy9ub2RlanMvdGVzdGRhdGEvc2VydmljZXMvc2ltcGxl", "base64")),
			i2.newPostServiceClient,
			context),
	}),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps, registrar: GrpcRegistrar) => Promise<void> | void;
export const wireService: WireService = impl.wireService;

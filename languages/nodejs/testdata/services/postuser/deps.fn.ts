// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer, Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-grpc/deps.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/service_grpc_pb"


export interface ServiceDeps {
	postService: i1.PostServiceClient;
}

export const Package = {
  name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/postuser",
  // Package dependencies are instantiated at most once.
  instantiateDeps: (graph: DependencyGraph) => ({
		postService: i0.BackendProvider(
			graph,
			i1.PostServiceClient),
  }),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps, registrar: Registrar) => Promise<void>;
export const wireService: WireService = impl.wireService;

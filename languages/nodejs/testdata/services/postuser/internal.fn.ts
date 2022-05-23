// This file was automatically generated.
// Contains Foundation-internal wiring, the user doesn't interact directly with it.

import * as impl from "./impl";
import * as api from "./api.fn";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-grpc/internal.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/service_grpc_pb"

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

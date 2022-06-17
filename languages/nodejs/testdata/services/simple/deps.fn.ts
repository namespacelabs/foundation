// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import {GrpcRegistrar} from "@namespacelabs.dev-foundation/std-nodejs-grpc"
import * as i0 from "@namespacelabs.dev-foundation/std-secrets/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/std-secrets/provider_pb";


export interface ServiceDeps {
	testSecrets: Promise<i1.Value>;
}

export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		testSecrets: i0.SecretProvider(
			graph,
			// name: "test-name"
			// generate: {
			//   random_byte_count: 32
			//   format: FORMAT_BASE64
			// }
			i1.Secret.deserializeBinary(Buffer.from("Cgl0ZXN0LW5hbWUaBBAgGAE=", "base64"))),
	}),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps, registrar: GrpcRegistrar) => Promise<void> | void;
export const wireService: WireService = impl.wireService;

// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import {GrpcRegistrar} from "@namespacelabs.dev-foundation/std-nodejs-grpc"
import * as i0 from "@namespacelabs.dev-foundation/std-secrets/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/std-secrets/provider_pb";
import * as i2 from "@namespacelabs.dev-foundation/std-secrets/impl";


export interface ServiceDeps {
	cert: i2.Value2;
}

export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		cert: i0.SecretProvider(
			graph,
			// name: "cert"
			i1.Secret.fromBinary(Buffer.from("CgRjZXJ0", "base64"))),
	}),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps, registrar: GrpcRegistrar) => Promise<void> | void;
export const wireService: WireService = impl.wireService;

// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/input_pb";
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/formatter";
import * as i3 from "@namespacelabs.dev-foundation/std-nodejs-grpc/deps.fn";
import * as i4 from "@namespacelabs.dev-foundation/std-nodejs-grpc/provider_pb";
import * as i5 from "@namespacelabs.dev-foundation/std-nodejs-grpc/registrar";


export interface ServiceDeps {
	batch1: Promise<i2.BatchFormatter>;
	batch2: Promise<i2.BatchFormatter>;
	grpcRegistrar: i5.GrpcRegistrar;
}

export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/numberformatter",
	// Package dependencies are instantiated at most once.
	instantiateDeps: (graph: DependencyGraph) => ({
		batch1: i0.BatchFormatterProvider(
			graph,
			i1.InputData.deserializeBinary(Buffer.from("", "base64"))),
		batch2: i0.BatchFormatterProvider(
			graph,
			i1.InputData.deserializeBinary(Buffer.from("", "base64"))),
		grpcRegistrar: i3.GrpcRegistrarProvider(
			graph,
			i4.NoArgs.deserializeBinary(Buffer.from("", "base64"))),
	}),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
	...i3.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps) => Promise<void>;
export const wireService: WireService = impl.wireService;

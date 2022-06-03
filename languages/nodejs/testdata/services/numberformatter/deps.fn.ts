// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer, Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/deps.fn";
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/input_pb";
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/formatter";


export interface ServiceDeps {
	batch1: Promise<i2.BatchFormatter>;
	batch2: Promise<i2.BatchFormatter>;
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
	}),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps, registrar: Registrar) => Promise<void> | void;
export const wireService: WireService = impl.wireService;

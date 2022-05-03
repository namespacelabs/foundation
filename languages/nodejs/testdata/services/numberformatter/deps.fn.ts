// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/deps.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/input_pb"
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/formatter"
import * as i3 from "@grpc/grpc-js"


export interface ServiceDeps {
	batch1: i2.BatchFormatter;
	batch2: i2.BatchFormatter;
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

export type WireService = (deps: ServiceDeps, server: i3.Server) => void;
export const wireService: WireService = impl.wireService;

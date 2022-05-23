// This file was automatically generated.
// Contains Foundation-internal wiring, the user doesn't interact directly with it.

import * as impl from "./impl";
import * as api from "./api.fn";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/internal.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/input_pb"
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/formatter"

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

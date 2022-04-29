// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev/foundation_languages_nodejs_testdata_extensions_numberformatter/deps.fn"
import * as i1 from "@namespacelabs.dev/foundation_languages_nodejs_testdata_extensions_numberformatter/input_pb"
import * as i2 from "@namespacelabs.dev/foundation_languages_nodejs_testdata_extensions_numberformatter/formatter"
import * as i3 from "@grpc/grpc-js"


export interface ServiceDeps {
	fmt: i2.NumberFormatter;
}

export const Package = {
  name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/numberformatter",
  // Package dependencies are instantiated at most once.
  instantiateDeps: (graph: DependencyGraph) => ({
		fmt: i0.FmtProvider(
			graph,
			// precision: 3
			i1.FormattingSettings.deserializeBinary(Buffer.from("CAM=", "base64"))),
  }),
};

export const TransitiveInitializers: Initializer[] = [
	...i0.TransitiveInitializers,
];

export type WireService = (deps: ServiceDeps, server: i3.Server) => void;
export const wireService: WireService = impl.wireService;

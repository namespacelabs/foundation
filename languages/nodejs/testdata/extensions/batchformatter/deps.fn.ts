// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer, Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/deps.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/input_pb"
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/formatter"
import * as i3 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/input_pb"
import * as i4 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/formatter"


export interface ExtensionDeps {
	fmt: i2.NumberFormatter;
}

export const Package = {
  name: "namespacelabs.dev/foundation/languages/nodejs/testdata/extensions/batchformatter",
  // Package dependencies are instantiated at most once.
  instantiateDeps: (graph: DependencyGraph) => ({
		fmt: i0.FmtProvider(
			graph,
			// precision: 2
			i1.FormattingSettings.deserializeBinary(Buffer.from("CAI=", "base64"))),
  }),
};

const initializer = {
  package: Package,
	initialize: impl.initialize,
};

export type Prepare = (deps: ExtensionDeps) => void;
export const prepare: Prepare = impl.initialize;

export const TransitiveInitializers: Initializer[] = [
	initializer,
	...i0.TransitiveInitializers,
];

export interface BatchFormatterDeps {
	fmt: i2.NumberFormatter;
}

export const BatchFormatterProvider = (graph: DependencyGraph, input: i3.InputData) =>
	provideBatchFormatter(
		input,
		graph.instantiatePackageDeps(Package),
		// Scoped dependencies that are instantiated for each call to ProvideBatchFormatter.
		graph.instantiateDeps(Package.name, "BatchFormatter", () => ({
		fmt: i0.FmtProvider(
			graph,
			// precision: 5
			i1.FormattingSettings.deserializeBinary(Buffer.from("CAU=", "base64"))),
  }))
  );

export type ProvideBatchFormatter = (input: i3.InputData, packageDeps: ExtensionDeps, deps: BatchFormatterDeps) =>
		i4.Promise<BatchFormatter>;
export const provideBatchFormatter: ProvideBatchFormatter = impl.provideBatchFormatter;

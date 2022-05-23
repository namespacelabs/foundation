// This file was automatically generated.
// Contains type and function definitions that needs to be implemented in "impl.ts".

import * as impl from "./impl";
import { Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/internal.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/input_pb"
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/formatter"
import * as i3 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/input_pb"
import * as i4 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/formatter"

export interface ExtensionDeps {
	fmt: i2.NumberFormatter;
}

export type Prepare = (deps: ExtensionDeps) => void;
export const prepare: Prepare = impl.initialize;

export interface BatchFormatterDeps {
	fmt: i2.NumberFormatter;
}

export type ProvideBatchFormatter = (input: i3.InputData, packageDeps: ExtensionDeps, deps: BatchFormatterDeps) =>
		i4.BatchFormatter;
export const provideBatchFormatter: ProvideBatchFormatter = impl.provideBatchFormatter;

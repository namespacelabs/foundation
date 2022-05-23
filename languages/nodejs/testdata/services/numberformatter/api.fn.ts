// This file was automatically generated.
// Contains type and function definitions that needs to be implemented in "impl.ts".

import * as impl from "./impl";
import { Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/internal.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/input_pb"
import * as i2 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-batchformatter/formatter"

export interface ServiceDeps {
	batch1: i2.BatchFormatter;
	batch2: i2.BatchFormatter;
}

export type WireService = (deps: ServiceDeps, registrar: Registrar) => void;
export const wireService: WireService = impl.wireService;

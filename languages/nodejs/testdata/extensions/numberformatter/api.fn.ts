// This file was automatically generated.
// Contains type and function definitions that needs to be implemented in "impl.ts".

import * as impl from "./impl";
import { Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/input_pb"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/formatter"

export type ProvideFmt = (input: i0.FormattingSettings) =>
		i1.NumberFormatter;
export const provideFmt: ProvideFmt = impl.provideFmt;

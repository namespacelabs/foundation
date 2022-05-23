// This file was automatically generated.
// Contains Foundation-internal wiring, the user doesn't interact directly with it.

import * as impl from "./impl";
import * as api from "./api.fn";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/input_pb"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/formatter"

export const Package = {
  name: "namespacelabs.dev/foundation/languages/nodejs/testdata/extensions/numberformatter",
};

export const TransitiveInitializers: Initializer[] = [
];

export const FmtProvider = (graph: DependencyGraph, input: i0.FormattingSettings) =>
	api.provideFmt(
		input
  );

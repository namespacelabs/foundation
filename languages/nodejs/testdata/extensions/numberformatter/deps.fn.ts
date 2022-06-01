// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer, Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/input_pb";
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-extensions-numberformatter/formatter";



export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/extensions/numberformatter",
};

export const TransitiveInitializers: Initializer[] = [
];


export const FmtProvider = (graph: DependencyGraph, input: i0.FormattingSettings) =>
	provideFmt(
		input
	);

export type ProvideFmt = (input: i0.FormattingSettings) =>
		i1.NumberFormatter;
export const provideFmt: ProvideFmt = impl.provideFmt;

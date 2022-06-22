// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { InstantiationContext } from "../../../../../std/nodejs/runtime";
import { NumberFormatter } from "./formatter";
import { FormattingSettings } from "./input_pb";

export function provideFmt(
	input: FormattingSettings,
	context: InstantiationContext
): NumberFormatter {
	let index = 0;
	console.log(`number formatter: instantiation path: ${JSON.stringify(context.path.packages)}`);
	return {
		formatNumber(n: number): string {
			return `Formatted value: ${n.toFixed(input.precision)}. This is called ${++index} times.`;
		},
	};
}

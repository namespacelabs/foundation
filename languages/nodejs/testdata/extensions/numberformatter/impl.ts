// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { NumberFormatter } from "./formatter";
import { FormattingSettings } from "./input_pb";

export function provideFmt(input: FormattingSettings): NumberFormatter {
	return {
		formatNumber(n: number): string {
			return n.toFixed(input.getPrecision());
		},
	};
}

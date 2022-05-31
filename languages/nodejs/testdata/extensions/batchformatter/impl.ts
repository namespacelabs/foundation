// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { BatchFormatterDeps, ExtensionDeps, ProvideBatchFormatter } from "./deps.fn";
import { BatchFormatter } from "./formatter";
import { InputData } from "./input_pb";

export const provideBatchFormatter: ProvideBatchFormatter = async (
	_: InputData,
	extensionDeps: ExtensionDeps,
	providerDeps: BatchFormatterDeps
): Promise<BatchFormatter> => ({
	getFormatResult: (n: number) => ({
		singleton: extensionDeps.fmt.formatNumber(n),
		scoped: providerDeps.fmt.formatNumber(n),
	}),
});

export function initialize(deps: ExtensionDeps) {
	console.warn(`BatchFormatter extension is initialized.`);
}

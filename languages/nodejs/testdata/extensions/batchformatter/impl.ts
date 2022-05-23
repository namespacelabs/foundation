// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { BatchFormatterDeps, ExtensionDeps, ProvideBatchFormatter } from "./api.fn";
import { BatchFormatter } from "./formatter";
import { InputData } from "./input_pb";

export const provideBatchFormatter: ProvideBatchFormatter = (
	_: InputData,
	extensionDeps: ExtensionDeps,
	providerDeps: BatchFormatterDeps
): BatchFormatter => ({
	getFormatResult: (n: number) => ({
		singleton: extensionDeps.fmt.formatNumber(n),
		scoped: providerDeps.fmt.formatNumber(n),
	}),
});

export function initialize(deps: ExtensionDeps) {
	console.warn(`BatchFormatter extension is initialized.`);
}

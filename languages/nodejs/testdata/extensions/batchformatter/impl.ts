import { BatchFormatterDeps, ExtensionDeps, ProvideBatchFormatter } from "./deps.fn";
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

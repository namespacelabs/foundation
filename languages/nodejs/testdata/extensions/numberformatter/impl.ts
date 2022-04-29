import { NumberFormatter } from "./formatter";
import { FormattingSettings } from "./input_pb";

export function provideFmt(input: FormattingSettings): NumberFormatter {
	return {
		formatNumber(n: number): string {
			return n.toFixed(input.getPrecision());
		},
	};
}

export interface BatchFormatter {
	getFormatResult(n: number): {
		singleton: string;
		scoped: string;
	};
}

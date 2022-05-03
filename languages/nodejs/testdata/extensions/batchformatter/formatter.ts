// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

export interface BatchFormatter {
	getFormatResult(n: number): {
		singleton: string;
		scoped: string;
	};
}

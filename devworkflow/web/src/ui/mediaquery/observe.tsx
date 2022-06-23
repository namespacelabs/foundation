// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useEffect, useState } from "react";

export function useMediaQuery(query: string) {
	let [state, setState] = useState<boolean | null>(null);

	useEffect(() => {
		let m = window.matchMedia(query);
		setState(m.matches);
		let l = (m: MediaQueryListEvent) => {
			setState(m.matches);
		};
		m.addEventListener("change", l);
		return () => {
			m.removeEventListener("change", l);
		};
	});

	return state;
}

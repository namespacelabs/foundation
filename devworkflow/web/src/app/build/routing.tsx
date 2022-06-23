// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { useRoute } from "wouter";

export function useBuildRoute() {
	return useRoute("/build/:what?");
}

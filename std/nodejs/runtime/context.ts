// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Package } from "./depgraph";

export interface InstantiationContext {
	path: InstantiationPath;
}

export interface InstantiationPath {
	// Call chain of this node, a list of package names.
	packages: string[];
}

export const EMPTY_CONTEXT: InstantiationContext = { path: { packages: [] } };

export function addPathToContext(
	context: InstantiationContext,
	pkgName: string
): InstantiationContext {
	return {
		path: {
			packages: [...context.path.packages, pkgName],
		},
	};
}

export function rootContextForPackage(pkg: Package<unknown>): InstantiationContext {
	return {
		path: {
			packages: [pkg.name],
		},
	};
}

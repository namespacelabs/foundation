// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { performance } from "perf_hooks";

const maximumInitTimeMs = 10;

export interface Package<PackageDepsT> {
	name: string;

	instantiateDeps: (dg: DependencyGraph) => PackageDepsT;
}

export class DependencyGraph {
	readonly #singletonDeps = new Map<string, unknown>();

	instantiatePackageDeps<PackageDepsT>(p: Package<PackageDepsT>): PackageDepsT {
		let deps = this.#singletonDeps.get(p.name) as PackageDepsT | undefined;
		if (!deps) {
			deps = this.profileCall(p.name, () => p.instantiateDeps(this));
			this.#singletonDeps.set(p.name, deps);
		}

		return deps;
	}

	profileCall<T>(loggingName: string, factory: () => T): T {
		const startMs = performance.now();
		const result = factory();
		const durationMs = performance.now() - startMs;
		if (durationMs > maximumInitTimeMs) {
			console.warn(
				`Generating dependencies of "${loggingName}" took ${durationMs.toFixed(
					3
				)}ms (log threshold is ${maximumInitTimeMs}).`
			);
		}
		return result;
	}
}

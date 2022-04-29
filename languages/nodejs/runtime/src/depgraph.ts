// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { performance } from "perf_hooks";

const maximumInitTimeMs = 10;

export interface Package<PackageDepsT> {
	name: string;

	instantiateDeps?: (dg: DependencyGraph) => PackageDepsT;
}

export interface Initializer<PackageDepsT> {
	package: Package<PackageDepsT>;
	// List of packages that need to be initialized before this package. Enforced at runtime.
	before?: string[];
	after?: string[];
	initialize: (deps: PackageDepsT) => void;
}

export class DependencyGraph {
	readonly #singletonDeps = new Map<string, unknown>();

	instantiatePackageDeps<PackageDepsT>(p: Package<PackageDepsT>): PackageDepsT {
		let deps = this.#singletonDeps.get(p.name) as PackageDepsT | undefined;
		if (!deps) {
			deps = this.#profileCall(`Generating dependencies of package "${p.name}"`, () =>
				p.instantiateDeps?.(this)
			);
			this.#singletonDeps.set(p.name, deps);
		}

		// It can be undefined if the package has no dependencies.
		return deps as PackageDepsT;
	}

	instantiateDeps<T>(pkgName: string, providerName: string, factory: () => T): T {
		return this.#profileCall(
			`Generating dependencies of provider "${pkgName}#${providerName}"`,
			factory
		);
	}

	runInitializers(initializers: unknown[]) {
		const dedupedInitializers = new Set(initializers);

		try {
			// TODO: take before/after into account
			dedupedInitializers.forEach((i) => this.#runInitializer(i as Initializer<unknown>));
		} catch (e) {
			console.error(`Error running initializers: ${e}`);
			process.exit(1);
		}
	}

	#runInitializer<PackageDepsT>(initializer: Initializer<PackageDepsT>) {
		this.#profileCall(`Initializing ${initializer.package.name}`, () => {
			initializer.initialize(this.instantiatePackageDeps(initializer.package));
		});
	}

	#profileCall<T>(loggingName: string, factory: () => T): T {
		const startMs = performance.now();
		const result = factory();
		const durationMs = performance.now() - startMs;
		if (durationMs > maximumInitTimeMs) {
			console.warn(
				`[${loggingName}] took ${durationMs.toFixed(3)}ms (log threshold is ${maximumInitTimeMs}).`
			);
		}
		return result;
	}
}

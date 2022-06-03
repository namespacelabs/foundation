// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { performance } from "perf_hooks";
import "source-map-support/register";
import toposort from "toposort";

const maximumInitTimeMs = 10;

interface Package<PackageDepsT> {
	name: string;

	instantiateDeps?: (dg: DependencyGraph) => PackageDepsT;
}

export interface Initializer {
	package: Package<any>;
	// List of packages that need to be initialized before this package. Enforced at runtime.
	before?: string[];
	after?: string[];
	initialize: (deps: any) => Promise<void> | void;
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

	async runInitializers(initializers: Initializer[]): Promise<void> {
		const initializerMap = new Map(initializers.map((i) => [i.package.name, i]));
		const edges: [string, string][] = [];
		initializers.forEach((i) => {
			if (i.before) {
				edges.push(...i.before.map((b) => [i.package.name, b] as [string, string]));
			}
			if (i.after) {
				edges.push(...i.after.map((a) => [a, i.package.name] as [string, string]));
			}
		});

		let sortedPackageNames: string[] | undefined;
		try {
			sortedPackageNames = toposort.array([...initializerMap.keys()], edges);
		} catch (e) {
			console.error(`Internal failure: initializer order not fulfillable: ${e}`);
			process.exit(1);
		}

		const dedupedInitializers = sortedPackageNames.map((name) => initializerMap.get(name)!);

		try {
			for (const initializer of dedupedInitializers) {
				await this.#profileAsyncCall(`Initializing ${initializer.package.name}`, async () => {
					await initializer.initialize(this.instantiatePackageDeps(initializer.package));
				});
			}
		} catch (e) {
			console.error(`Error running initializers: ${e}`);
			process.exit(1);
		}
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

	async #profileAsyncCall<T>(loggingName: string, factory: () => Promise<T>): Promise<T> {
		const startMs = performance.now();
		const result = await factory();
		const durationMs = performance.now() - startMs;
		if (durationMs > maximumInitTimeMs) {
			console.warn(
				`[${loggingName}] took ${durationMs.toFixed(3)}ms (log threshold is ${maximumInitTimeMs}).`
			);
		}
		return result;
	}
}

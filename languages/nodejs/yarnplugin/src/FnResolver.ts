// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import {
	Descriptor,
	LinkType,
	Locator,
	Manifest,
	MinimalResolveOptions,
	miscUtils,
	ResolveOptions,
	Resolver,
	structUtils,
} from "@yarnpkg/core";
import { PROTOCOL } from "./constants";

export class FnResolver implements Resolver {
	supportsDescriptor(descriptor: Descriptor, opts: MinimalResolveOptions) {
		return descriptor.range.startsWith(PROTOCOL);
	}

	supportsLocator(locator: Locator, opts: MinimalResolveOptions) {
		return locator.reference.startsWith(PROTOCOL);
	}

	shouldPersistResolution(locator: Locator, opts: MinimalResolveOptions) {
		return false;
	}

	bindDescriptor(descriptor: Descriptor, fromLocator: Locator, opts: MinimalResolveOptions) {
		return descriptor;
	}

	getResolutionDependencies(descriptor: Descriptor, opts: MinimalResolveOptions) {
		return [];
	}

	async getCandidates(descriptor: Descriptor, dependencies: unknown, opts: ResolveOptions) {
		if (!opts.fetchOptions) {
			throw new Error(
				`Assertion failed: This resolver cannot be used unless a fetcher is configured`
			);
		}

		return [structUtils.makeLocator(descriptor, descriptor.range)];
	}

	async getSatisfying(descriptor: Descriptor, references: Array<string>, opts: ResolveOptions) {
		return null;
	}

	async resolve(locator: Locator, opts: ResolveOptions) {
		if (!opts.fetchOptions)
			throw new Error(
				`Assertion failed: This resolver cannot be used unless a fetcher is configured`
			);

		const packageFetch = await opts.fetchOptions.fetcher.fetch(locator, opts.fetchOptions);

		const manifest = await miscUtils.releaseAfterUseAsync(async () => {
			return await Manifest.find(packageFetch.prefixPath, { baseFs: packageFetch.packageFs });
		}, packageFetch.releaseFs);

		return {
			...locator,

			version: manifest.version || `0.0.0`,

			languageName: manifest.languageName || opts.project.configuration.get(`defaultLanguageName`),
			linkType: LinkType.HARD,

			conditions: manifest.getConditions(),

			dependencies: manifest.dependencies,
			peerDependencies: manifest.peerDependencies,

			dependenciesMeta: manifest.dependenciesMeta,
			peerDependenciesMeta: manifest.peerDependenciesMeta,

			bin: manifest.bin,
		};
	}
}

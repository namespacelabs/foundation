// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import {
	Descriptor,
	hashUtils,
	LinkType,
	Locator,
	LocatorHash,
	Manifest,
	MinimalResolveOptions,
	miscUtils,
	ResolveOptions,
	Resolver,
	structUtils,
	tgzUtils,
} from "@yarnpkg/core";
import { CwdFS, PortablePath } from "@yarnpkg/fslib";
import { PROTOCOL } from "./constants";
import { FnFetcher } from "./FnFetcher";

// We use this for the folders to be regenerated without bumping the whole cache
const CACHE_VERSION = 2;

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

		const locator = structUtils.makeLocator(descriptor, descriptor.range);

		// Snapshotting the folder to make Yarn re-fetch it if the contents changes.
		const zipFs = await tgzUtils.makeArchiveFromDirectory(
			(opts.fetchOptions.fetcher as FnFetcher).getLocalPath(locator, opts.fetchOptions),
			{
				baseFs: new CwdFS(PortablePath.root),
				compressionLevel: opts.fetchOptions.project.configuration.get(`compressionLevel`),
				inMemory: true,
			}
		);
		const archiveBuffer = zipFs.getBufferAndClose();

		// This is copied from the "file" Yarn plugin.
		const folderHash = hashUtils.makeHash(`${CACHE_VERSION}`, archiveBuffer).slice(0, 6);

		const reference = `${locator.reference}::${folderHash}`;

		return [
			{
				...locator,
				locatorHash: hashUtils.makeHash<LocatorHash>(locator.identHash, reference),
				reference,
			},
		];
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

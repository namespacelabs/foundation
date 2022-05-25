// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import {
	Fetcher,
	FetchOptions,
	Locator,
	MinimalFetchOptions,
	structUtils,
	tgzUtils,
} from "@yarnpkg/core";
import { CwdFS, PortablePath } from "@yarnpkg/fslib";
import { readFileSync } from "fs";
import path from "path";
import { LOCK_FILE_PATH, PROTOCOL } from "./constants";

export class FnFetcher implements Fetcher {
	readonly #modules: { [key: string]: { path: string } };

	constructor() {
		const lockFileBytes = readFileSync(LOCK_FILE_PATH, "utf8");
		const lockFile = JSON.parse(lockFileBytes);
		this.#modules = lockFile.modules || {};
	}

	supports(locator: Locator, opts: MinimalFetchOptions) {
		return locator.reference.startsWith(PROTOCOL);
	}

	getLocalPath(locator: Locator, opts: FetchOptions) {
		const { selector: packageName } = structUtils.parseRange(locator.reference);
		if (!packageName) {
			throw new Error(`locator.reference can't be parsed: ${locator.reference}`);
		}

		for (const [moduleName, module] of Object.entries(this.#modules)) {
			if (packageName.startsWith(moduleName)) {
				const relPath = packageName.substring(moduleName.length);
				const resolvedPath = path.join(module.path, relPath);
				return resolvedPath as PortablePath;
			}
		}
		throw new Error(
			`Package "${packageName}" couldn't be resolved. Known modules:\n${JSON.stringify(
				this.#modules,
				undefined,
				2
			)}`
		);
	}

	async fetch(locator: Locator, opts: FetchOptions) {
		const expectedChecksum = opts.checksums.get(locator.locatorHash) || null;

		const [packageFs, releaseFs, checksum] = await opts.cache.fetchPackageFromCache(
			locator,
			expectedChecksum,
			{
				onHit: () => opts.report.reportCacheHit(locator),
				onMiss: () =>
					opts.report.reportCacheMiss(
						locator,
						`${structUtils.prettyLocator(
							opts.project.configuration,
							locator
						)} can't be found in the cache and will be fetched from the disk`
					),
				loader: () => this.fetchFromDisk(locator, opts),
				skipIntegrityCheck: opts.skipIntegrityCheck,
				...opts.cacheOptions,
			}
		);

		return {
			packageFs,
			releaseFs,
			prefixPath: "" as PortablePath,
			localPath: this.getLocalPath(locator, opts),
			checksum,
		};
	}

	private async fetchFromDisk(locator: Locator, fetchOptions: FetchOptions) {
		return tgzUtils.makeArchiveFromDirectory(this.getLocalPath(locator, fetchOptions), {
			baseFs: new CwdFS(PortablePath.root),
			compressionLevel: fetchOptions.project.configuration.get(`compressionLevel`),
		});
	}
}

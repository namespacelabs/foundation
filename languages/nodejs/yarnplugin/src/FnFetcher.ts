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
import * as path from "path";
import { PROTOCOL } from "./constants";

export class FnFetcher implements Fetcher {
	supports(locator: Locator, opts: MinimalFetchOptions) {
		return locator.reference.startsWith(PROTOCOL);
	}

	getLocalPath(locator: Locator, opts: FetchOptions) {
		// Process the file path to point to the Foundation cache location.
		const basePath = locator.reference.replace(PROTOCOL, "");
		const fnModuleCacheDir = process.env.FN_MODULE_CACHE;
		if (!fnModuleCacheDir) {
			throw new Error(
				`Foundation Yarn plugin: $FN_MODULE_CACHE is not defined. Please avoid running Yarn manually, use "fn tidy" instead.`
			);
		}
		return path.join(fnModuleCacheDir, basePath) as PortablePath;
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

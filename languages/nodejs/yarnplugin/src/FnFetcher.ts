// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Fetcher, FetchOptions, Locator, MinimalFetchOptions, structUtils } from "@yarnpkg/core";
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
		const path = this.getLocalPath(locator, opts);

		return {
			packageFs: new CwdFS(path),
			prefixPath: PortablePath.dot,
			localPath: path,
		};
	}
}

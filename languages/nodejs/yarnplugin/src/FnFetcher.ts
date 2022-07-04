// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import {
	Fetcher,
	FetchOptions,
	FetchResult,
	Locator,
	MinimalFetchOptions,
	structUtils,
} from "@yarnpkg/core";
import { CwdFS, PortablePath, ppath } from "@yarnpkg/fslib";
import { readFileSync } from "fs";
import { LOCK_FILE_PATH_ENV, PROTOCOL } from "./constants";

export class FnFetcher implements Fetcher {
	readonly #rootPath: PortablePath;
	readonly #modules: { [key: string]: { path: string } };

	constructor() {
		const lockFn = process.env[LOCK_FILE_PATH_ENV];
		if (!lockFn) {
			throw new Error(`Lock file can't be found: ${LOCK_FILE_PATH_ENV} is not set.`);
		}
		const lockFileBytes = readFileSync(lockFn, "utf8");
		const lockFile = JSON.parse(lockFileBytes);
		console.log(`lockFile: ${JSON.stringify(lockFile, undefined, 2)}`);
		this.#modules = lockFile.modules || {};
		this.#rootPath = lockFile.rootPath;
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
			if (packageName == moduleName) {
				const resolvedPath = ppath.resolve(this.#rootPath, module.path as PortablePath);
				console.log(`resolvedPath: ${resolvedPath}`);
				return resolvedPath;
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

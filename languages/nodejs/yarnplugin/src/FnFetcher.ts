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
	readonly #modules: { [key: string]: { path: string } };

	constructor() {
		const lockFn = process.env[LOCK_FILE_PATH_ENV];
		if (!lockFn) {
			throw new Error(`Lock file can't be found: ${LOCK_FILE_PATH_ENV} is not set.`);
		}
		let lockFileBytes = "{}";
		try {
			lockFileBytes = readFileSync(lockFn, "utf8");
		} catch (e) {
			// File can be not there for WEB at the moment.
			// TODO: remove catching the exception once WEB is unified with NODEJS.
		}
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
			if (packageName == moduleName) {
				return module.path as PortablePath;
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

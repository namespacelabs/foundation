// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { Fetcher, Locator, structUtils } from "@yarnpkg/core";
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
		const lockFileBytes = readFileSync(lockFn, "utf8");
		const lockFile = JSON.parse(lockFileBytes);
		this.#modules = lockFile.modules || {};
	}

	supports(locator: Locator) {
		return locator.reference.startsWith(PROTOCOL);
	}

	getLocalPath(locator: Locator) {
		const { selector: packageName } = structUtils.parseRange(locator.reference);
		if (!packageName) {
			throw new Error(`locator.reference can't be parsed: ${locator.reference}`);
		}

		for (const [moduleName, module] of Object.entries(this.#modules)) {
			if (packageName.startsWith(moduleName)) {
				const relPath = packageName.slice(moduleName.length);
				return ppath.join(
					ppath.resolve(process.cwd() as PortablePath, module.path as PortablePath),
					relPath as PortablePath
				);
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

	async fetch(locator: Locator) {
		const path = this.getLocalPath(locator);

		return {
			packageFs: new CwdFS(path),
			prefixPath: PortablePath.dot,
			localPath: path,
		};
	}
}

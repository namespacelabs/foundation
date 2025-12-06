// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

const fs = require("fs");
const os = require("os");
const path = require("path");
const { install, fetchVersionInfo } = require("../lib/install");

describe("install", () => {
	let tmpDir;
	let originalNsRoot;

	beforeEach(() => {
		tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "nsc-test-"));
		originalNsRoot = process.env.NS_ROOT;
		process.env.NS_ROOT = tmpDir;
	});

	afterEach(() => {
		if (originalNsRoot) {
			process.env.NS_ROOT = originalNsRoot;
		} else {
			delete process.env.NS_ROOT;
		}
		fs.rmSync(tmpDir, { recursive: true, force: true });
	});

	test("fetchVersionInfo returns valid version info", async () => {
		const versionInfo = await fetchVersionInfo();

		expect(versionInfo.version).toBeDefined();
		expect(versionInfo.version).toMatch(/^v\d+\.\d+\.\d+$/);
		expect(versionInfo.url).toBeDefined();
		expect(versionInfo.sha256).toBeDefined();
		expect(versionInfo.sha256).toMatch(/^[a-f0-9]{64}$/);
	});

	test("downloads and installs latest version", async () => {
		const versionInfo = await fetchVersionInfo();
		const result = await install({ force: true, versionInfo });

		expect(result.alreadyInstalled).toBe(false);
		expect(result.path).toBeDefined();
		expect(fs.existsSync(result.path)).toBe(true);

		const stat = fs.statSync(result.path);
		expect(stat.isFile()).toBe(true);
		expect(stat.size).toBeGreaterThan(0);
	}, 60000);
});

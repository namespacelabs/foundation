// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

const https = require("https");
const http = require("http");
const crypto = require("crypto");
const fs = require("fs");
const path = require("path");
const os = require("os");
const { Readable } = require("stream");
const tar = require("tar");

const TOOL_NAME = "nsc";
const DOCKER_CRED_HELPER_NAME = "docker-credential-nsc";
const BAZEL_CRED_HELPER_NAME = "bazel-credential-nsc";
const API_ENDPOINT = "https://get.namespace.so/nsl.versions.VersionsService/GetLatest";

function getPlatform() {
	const platform = os.platform();
	switch (platform) {
		case "darwin":
			return "darwin";
		case "linux":
			return "linux";
		default:
			throw new Error(`Unsupported platform: ${platform}. nsc is available for macOS and Linux.`);
	}
}

function getArch() {
	const arch = os.arch();
	switch (arch) {
		case "x64":
			return "amd64";
		case "arm64":
			return "arm64";
		default:
			throw new Error(`Unsupported architecture: ${arch}. nsc is available for amd64 and arm64.`);
	}
}

function getNsRoot() {
	if (process.env.NS_ROOT) {
		return process.env.NS_ROOT;
	}
	const platform = os.platform();
	const home = os.homedir();
	if (platform === "darwin") {
		return path.join(home, "Library", "Application Support", "ns");
	}
	return path.join(home, ".ns");
}

function getBinDir() {
	return path.join(getNsRoot(), "bin");
}

function getBinaryPath() {
	return path.join(getBinDir(), TOOL_NAME);
}

function getDockerCredHelperPath() {
	return path.join(getBinDir(), DOCKER_CRED_HELPER_NAME);
}

function getBazelCredHelperPath() {
	return path.join(getBinDir(), BAZEL_CRED_HELPER_NAME);
}

function isInstalled() {
	return fs.existsSync(getBinaryPath());
}

function getVersionFilePath() {
	return path.join(getBinDir(), ".version");
}

function getInstalledVersion() {
	const versionFile = getVersionFilePath();
	if (fs.existsSync(versionFile)) {
		return fs.readFileSync(versionFile, "utf8").trim();
	}
	return null;
}

function setInstalledVersion(version) {
	fs.writeFileSync(getVersionFilePath(), version);
}

function httpRequest(url, options = {}) {
	return new Promise((resolve, reject) => {
		const doRequest = (requestUrl, method, body) => {
			const urlObj = new URL(requestUrl);
			const protocol = urlObj.protocol === "https:" ? https : http;
			const reqOptions = {
				hostname: urlObj.hostname,
				port: urlObj.port || (urlObj.protocol === "https:" ? 443 : 80),
				path: urlObj.pathname + urlObj.search,
				method: method,
				headers: {
					"User-Agent": "@namespacelabs/cli",
					...options.headers,
				},
			};

			const req = protocol.request(reqOptions, (response) => {
				if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
					let redirectUrl = response.headers.location;
					if (redirectUrl.startsWith("/")) {
						redirectUrl = `${urlObj.protocol}//${urlObj.host}${redirectUrl}`;
					}
					doRequest(redirectUrl, "GET", null);
					return;
				}

				if (response.statusCode !== 200) {
					reject(new Error(`HTTP ${response.statusCode}`));
					return;
				}

				const chunks = [];
				response.on("data", (chunk) => chunks.push(chunk));
				response.on("end", () => resolve({ data: Buffer.concat(chunks), response }));
				response.on("error", reject);
			});

			req.on("error", reject);

			if (body) {
				req.write(body);
			}
			req.end();
		};

		doRequest(url, options.method || "GET", options.body || null);
	});
}

async function fetchVersionInfo() {
	const platform = getPlatform().toUpperCase();
	const arch = getArch().toUpperCase();

	const { data } = await httpRequest(API_ENDPOINT, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ [TOOL_NAME]: {} }),
	});

	const response = JSON.parse(data.toString());
	const tarball = response.tarballs.find(
		(t) => t.os === platform && t.arch === arch
	);

	if (!tarball) {
		throw new Error(`No tarball found for ${platform}/${arch}`);
	}

	return {
		version: response.version,
		buildTime: response.build_time,
		url: tarball.url,
		sha256: tarball.sha256,
	};
}

async function download(url) {
	const { data } = await httpRequest(url);
	return data;
}

function verifySha256(buffer, expectedHash) {
	const hash = crypto.createHash("sha256").update(buffer).digest("hex");
	if (hash !== expectedHash) {
		throw new Error(`SHA256 mismatch: expected ${expectedHash}, got ${hash}`);
	}
}

async function install(options = {}) {
	const { force = false, versionInfo = null } = options;
	const binDir = getBinDir();

	if (!force && isInstalled()) {
		return { alreadyInstalled: true, path: getBinaryPath() };
	}

	const info = versionInfo || (await fetchVersionInfo());

	if (!fs.existsSync(binDir)) {
		fs.mkdirSync(binDir, { recursive: true });
	}

	console.error(`Downloading nsc ${info.version} from ${info.url}...`);

	const tarGzBuffer = await download(info.url);

	verifySha256(tarGzBuffer, info.sha256);

	await new Promise((resolve, reject) => {
		const stream = Readable.from(tarGzBuffer);
		stream
			.pipe(
				tar.x({
					cwd: binDir,
					filter: (p) =>
						p === TOOL_NAME ||
						p === DOCKER_CRED_HELPER_NAME ||
						p === BAZEL_CRED_HELPER_NAME,
					gzip: true,
				})
			)
			.on("finish", resolve)
			.on("error", reject);
	});

	fs.chmodSync(path.join(binDir, TOOL_NAME), 0o755);
	if (fs.existsSync(path.join(binDir, DOCKER_CRED_HELPER_NAME))) {
		fs.chmodSync(path.join(binDir, DOCKER_CRED_HELPER_NAME), 0o755);
	}
	if (fs.existsSync(path.join(binDir, BAZEL_CRED_HELPER_NAME))) {
		fs.chmodSync(path.join(binDir, BAZEL_CRED_HELPER_NAME), 0o755);
	}

	setInstalledVersion(info.version);

	console.error(`nsc ${info.version} installed successfully to ${getBinaryPath()}`);

	return { alreadyInstalled: false, path: getBinaryPath(), version: info.version };
}

async function update() {
	const installedVersion = getInstalledVersion();
	const versionInfo = await fetchVersionInfo();

	if (installedVersion && installedVersion === versionInfo.version) {
		return { alreadyLatest: true, version: installedVersion };
	}

	console.error("Updating nsc to the latest version...");
	return install({ force: true, versionInfo });
}

module.exports = {
	install,
	update,
	isInstalled,
	getBinaryPath,
	getDockerCredHelperPath,
	getBazelCredHelperPath,
	getInstalledVersion,
	getBinDir,
	fetchVersionInfo,
};

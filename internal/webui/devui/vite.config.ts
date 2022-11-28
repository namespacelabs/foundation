// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import pluginRewriteAll from "vite-plugin-rewrite-all";

export default ({ mode }) => {
	process.env = { ...process.env, ...loadEnv(mode, process.cwd()) };

	return defineConfig({
		plugins: [react(), pluginRewriteAll()],

		server: {
			hmr: {
				clientPort: process.env.CMD_DEV_PORT ? Number.parseInt(process.env.CMD_DEV_PORT) : undefined,
			},
		},
	});
};

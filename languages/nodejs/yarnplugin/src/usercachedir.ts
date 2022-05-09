// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import * as path from "path";

// Partial port from Go's "os.UserCacheDir".
export function UserCacheDir(): string {
	const home = process.env.HOME;
	switch (process.platform) {
		case "darwin": {
			if (!home) {
				throw new Error(`Foundation plugin: $HOME is not defined`);
			}
			return path.join(home, "Library", "Caches");
		}
		default: {
			// Unix
			const cacheHome = process.env.XDG_CACHE_HOME || process.env.HOME;
			if (!cacheHome) {
				throw new Error(`Foundation plugin: neither $XDG_CACHE_HOME nor $HOME are defined`);
			}
			return path.join(cacheHome, ".cache");
		}
	}
}

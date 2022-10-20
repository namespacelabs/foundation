// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Plugin } from "@yarnpkg/core";
import { FnFetcher } from "./FnFetcher";
import { FnResolver } from "./FnResolver";

const plugin: Plugin = {
	hooks: {
		afterAllInstalled: () => {
			console.log(`Foundation plugin installed`);
		},
	},
	fetchers: [FnFetcher],
	resolvers: [FnResolver],
};

export default plugin;

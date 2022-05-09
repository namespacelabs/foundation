import { Plugin } from "@yarnpkg/core";
import { FnFetcher } from "./FnFetcher";
import { FnResolver } from "./FnResolver";

const plugin: Plugin = {
	hooks: {
		afterAllInstalled: () => {
			console.log(`Foundation plugin is installed`);
		},
	},
	fetchers: [FnFetcher],
	resolvers: [FnResolver],
};

export default plugin;

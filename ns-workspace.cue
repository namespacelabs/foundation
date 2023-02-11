module: "namespacelabs.dev/foundation"
requirements: {
	api:          54
	toolsVersion: 4
}
prebuilts: {
	digest: {
		"namespacelabs.dev/foundation/internal/sdk/buf/image/prebuilt":     "sha256:5a9f9711fcd93aa2cdb5d2ee2aaa1b2fdd23b7e139ff4a39438153668b9b84ef"
		"namespacelabs.dev/foundation/std/development/filesync/controller": "sha256:41ffa681aec6a70dcd5a7ebeccd94814688389a45f39810138a4d3f1ef8278da"
	}
	baseRepository: "us-docker.pkg.dev/foundation-344819/prebuilts/"
}
internalAliases: [{
	module_name: "library.namespace.so"
	rel_path:    "library"
}]

enabledFeatures: ["experimental/container/annotations"]

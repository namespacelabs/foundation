module: "namespacelabs.dev/foundation"
requirements: {
	api:          81
	toolsVersion: 4
}
prebuilts: {
	digest: {
		"namespacelabs.dev/foundation/std/development/filesync/controller": "sha256:41ffa681aec6a70dcd5a7ebeccd94814688389a45f39810138a4d3f1ef8278da"
	}
	baseRepository: "us-docker.pkg.dev/foundation-344819/prebuilts/"
}
internalAliases: [{
	module_name: "library.namespace.so"
	rel_path:    "library"
}]
enabledFeatures: ["experimental/container/annotations"]
environment: {
	dev: {
		runtime: "kubernetes"
		purpose: "DEVELOPMENT"
	}
	staging: {
		runtime: "kubernetes"
		purpose: "PRODUCTION"
	}
	prod: {
		runtime: "kubernetes"
		purpose: "PRODUCTION"
		policy: {
			require_deployment_reason: true
		}
	}
}

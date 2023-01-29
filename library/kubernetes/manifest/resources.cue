resourceClasses: {
	"Manifest": {
		description: "Postgres Database Cluster"
		produces: {
			type:   "library.kubernetes.manifest.ManifestInstance"
			source: "./types.proto"
		}
	}
}

providers: {
	"namespacelabs.dev/foundation/library/kubernetes/manifest:Manifest": {
		prepareWith: "namespacelabs.dev/foundation/library/kubernetes/manifest/prepare"

		intent: {
			type:   "library.kubernetes.manifest.ManifestIntent"
			source: "./types.proto"
		}
	}
}

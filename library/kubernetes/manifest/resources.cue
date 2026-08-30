resourceClasses: {
	"AppliedManifest": {
		description: "Kubernetes Manifest"
		produces: {
			type:   "library.kubernetes.manifest.AppliedManifestInstance"
			source: "./types.proto"
		}
	}
}

providers: {
	"namespacelabs.dev/foundation/library/kubernetes/manifest:AppliedManifest": {
		prepareWith: "namespacelabs.dev/foundation/library/kubernetes/manifest/prepare"

		intent: {
			type:   "library.kubernetes.manifest.AppliedManifestIntent"
			source: "./types.proto"
		}
	}
}

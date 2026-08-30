resourceClasses: {
	"HelmRelease": {
		description: "Helm release from a static set of charts"
		produces: {
			type:   "library.kubernetes.helm.HelmReleaseInstance"
			source: "./types.proto"
		}
	}
}

providers: {
	"namespacelabs.dev/foundation/library/kubernetes/helm:HelmRelease": {
		prepareWith: "namespacelabs.dev/foundation/library/kubernetes/helm/prepare"

		intent: {
			type:   "library.kubernetes.helm.HelmReleaseIntent"
			source: "./types.proto"
		}
	}
}

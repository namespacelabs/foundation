resources: {
	existing: {
		class:    "namespacelabs.dev/foundation/library/kubernetes/manifest:AppliedManifest"
		provider: "namespacelabs.dev/foundation/library/kubernetes/manifest"
		intent: sources: ["foobar.yaml"]
	}

	redisHelm: {
		class:    "namespacelabs.dev/foundation/library/kubernetes/helm:HelmRelease"
		provider: "namespacelabs.dev/foundation/library/kubernetes/helm"
		intent: {
			releaseName: "redis"
			chart: fromURL: {
				url:    "https://charts.bitnami.com/bitnami/redis-17.6.0.tgz"
				digest: "sha256:7ea314d022bd783cdc0107e67a9bfbd771dc09107e9b4b08c3ca23c16e447a1d"
			}
			values: {
				replica: replicaCount: 1
			}
		}
	}
}

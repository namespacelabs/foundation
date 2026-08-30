resources: {
	colocated: {
		class:    "namespacelabs.dev/foundation/library/oss/localstack:Cluster"
		provider: "namespacelabs.dev/foundation/library/oss/localstack"

		intent: {
			server: "namespacelabs.dev/foundation/library/oss/localstack/server"
		}
	}
}

resources: {
	// colocated represents a cockroach cluster that can be easily used by multiple users, within a single cluster.
	colocated: {
		class:    "namespacelabs.dev/foundation/library/database/cockroach:Cluster"
		provider: "namespacelabs.dev/foundation/library/oss/cockroach"

		intent: {
			server:          "namespacelabs.dev/foundation/library/oss/cockroach/server"
			password_secret: "namespacelabs.dev/foundation/library/oss/cockroach/server:password"
		}
	}
}

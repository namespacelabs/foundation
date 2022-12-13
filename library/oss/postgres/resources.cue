resources: {
	// colocated represents a Postgres cluster that can be easily used by multiple users, within a single cluster.
	colocated: {
		class:    "namespacelabs.dev/foundation/library/database/postgres:Cluster"
		provider: "namespacelabs.dev/foundation/library/oss/postgres"

		intent: {
			server:          "namespacelabs.dev/foundation/library/oss/postgres/server"
			password_secret: "namespacelabs.dev/foundation/library/oss/postgres/server:password"
		}
	}
}

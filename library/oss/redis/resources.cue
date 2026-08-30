resources: {
	// colocated represents a Redis cluster that can be easily used by multiple users, within a single cluster.
	colocated: {
		class:    "namespacelabs.dev/foundation/library/database/redis:Cluster"
		provider: "namespacelabs.dev/foundation/library/oss/redis"

		intent: {
			server:          "namespacelabs.dev/foundation/library/oss/redis/server"
			password_secret: "namespacelabs.dev/foundation/library/oss/redis/server:password"
		}
	}
}

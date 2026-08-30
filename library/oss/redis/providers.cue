providers: {
	"namespacelabs.dev/foundation/library/database/redis:Database": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/redis/prepare/database"

		intent: {
			type:   "library.oss.redis.DatabaseIntent"
			source: "./types.proto"
		}

		inputs: {
			cluster: {
				class:   "namespacelabs.dev/foundation/library/database/redis:Cluster"
				default: ":colocated"
			}
		}
	}

	"namespacelabs.dev/foundation/library/database/redis:Cluster": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/redis/prepare/cluster"

		intent: {
			type:   "library.oss.redis.ClusterIntent"
			source: "./types.proto"
		}

		resourcesFrom: "namespacelabs.dev/foundation/library/oss/redis/prepare/clusterinstance"

		availableClasses: [
			"namespacelabs.dev/foundation/library/runtime:Server",
			"namespacelabs.dev/foundation/library/runtime:Secret",
		]

		availablePackages: [
			"namespacelabs.dev/foundation/library/oss/redis/server",
		]

	}
}

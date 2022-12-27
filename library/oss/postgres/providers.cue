providers: {
	"namespacelabs.dev/foundation/library/database/postgres:Database": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/postgres/prepare/database"

		intent: {
			type:   "library.oss.postgres.DatabaseIntent"
			source: "./types.proto"
		}

		inputs: {
			cluster: {
				class:   "namespacelabs.dev/foundation/library/database/postgres:Cluster"
				default: ":colocated"
			}
		}
	}

	"namespacelabs.dev/foundation/library/database/postgres:Cluster": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/postgres/prepare/cluster"

		intent: {
			type:   "library.oss.postgres.ClusterIntent"
			source: "./types.proto"
		}

		resourcesFrom: "namespacelabs.dev/foundation/library/oss/postgres/prepare/clusterinstance"

		availableClasses: [
			"namespacelabs.dev/foundation/library/runtime:Server",
			"namespacelabs.dev/foundation/library/runtime:Secret",
		]

		availablePackages: [
			"namespacelabs.dev/foundation/library/oss/postgres/server",
		]
	}
}

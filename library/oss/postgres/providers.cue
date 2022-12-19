providers: {
	"namespacelabs.dev/foundation/library/database/postgres:Database": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/postgres/prepare/database"

		inputs: {
			cluster: "namespacelabs.dev/foundation/library/database/postgres:Cluster"
		}

		defaults: {
			cluster: ":colocated"
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

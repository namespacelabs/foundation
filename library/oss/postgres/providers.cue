providers: {
	"namespacelabs.dev/foundation/library/database/postgres:Database": {
		initializedWith: imageFrom: binary: "namespacelabs.dev/foundation/library/oss/postgres/prepare/database"

		inputs: {
			// TODO define default
			"cluster": "namespacelabs.dev/foundation/library/database/postgres:Cluster"
		}
	}

	"namespacelabs.dev/foundation/library/database/postgres:Cluster": {
		initializedWith: imageFrom: binary: "namespacelabs.dev/foundation/library/oss/postgres/prepare/cluster"

		intent: {
			type:   "library.oss.postgres.ClusterIntent"
			source: "./types.proto"
		}

		resourcesFrom: imageFrom: binary: "namespacelabs.dev/foundation/library/oss/postgres/prepare/clusterinstance"

		availableClasses: [
			"namespacelabs.dev/foundation/library/runtime:Server",
			"namespacelabs.dev/foundation/library/runtime:Secret",
		]

		availablePackages: [
			"namespacelabs.dev/foundation/library/oss/postgres/server",
		]
	}
}

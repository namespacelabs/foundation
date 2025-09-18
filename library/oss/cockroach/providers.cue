providers: {
	"namespacelabs.dev/foundation/library/database/cockroach:Database": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/cockroach/prepare/database"

		intent: {
			type:   "library.oss.cockroach.DatabaseIntent"
			source: "./types.proto"
		}

		inputs: {
			cluster: {
				class:   "namespacelabs.dev/foundation/library/database/cockroach:Cluster"
				default: ":colocated"
			}
		}
	}

	"namespacelabs.dev/foundation/library/database/cockroach:Cluster": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/cockroach/prepare/cluster"

		intent: {
			type:   "library.oss.cockroach.ClusterIntent"
			source: "./types.proto"
		}

		resourcesFrom: "namespacelabs.dev/foundation/library/oss/cockroach/prepare/clusterinstance"

		availableClasses: [
			"namespacelabs.dev/foundation/library/runtime:Server",
			"namespacelabs.dev/foundation/library/runtime:Secret",
		]

		availablePackages: [
			"namespacelabs.dev/foundation/library/oss/cockroach/server",
		]
	}
}

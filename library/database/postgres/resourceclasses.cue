resourceClasses: {
	"Database": {
		intent: {
			type:   "library.database.postgres.DatabaseIntent"
			source: "./database.proto"
		}
		produces: {
			type:   "library.database.postgres.DatabaseInstance"
			source: "./database.proto"
		}
	}
	"Cluster": {
		intent: {
			type:   "library.database.postgres.ClusterIntent"
			source: "./cluster.proto"
		}
		produces: {
			type:   "library.database.postgres.ClusterInstance"
			source: "./cluster.proto"
		}
	}
}

resourceClasses: {
	"Database": {
		description: "CockroachDB Database"
		produces: {
			type:   "library.database.cockroach.DatabaseInstance"
			source: "./types.proto"
		}
	}
	"Cluster": {
		description: "CockroachDB Database Cluster"
		produces: {
			type:   "library.database.cockroach.ClusterInstance"
			source: "./types.proto"
		}
	}
}

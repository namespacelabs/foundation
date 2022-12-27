resourceClasses: {
	"Database": {
		description: "Postgres Database"
		produces: {
			type:   "library.database.postgres.DatabaseInstance"
			source: "./types.proto"
		}
	}
	"Cluster": {
		description: "Postgres Database Cluster"
		produces: {
			type:   "library.database.postgres.ClusterInstance"
			source: "./types.proto"
		}
	}
}

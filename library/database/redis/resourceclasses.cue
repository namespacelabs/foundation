resourceClasses: {
	"Database": {
		description: "Redis Database"
		produces: {
			type:   "library.database.redis.DatabaseInstance"
			source: "./types.proto"
		}
	}
	"Cluster": {
		description: "Redis Database Cluster"
		produces: {
			type:   "library.database.redis.ClusterInstance"
			source: "./types.proto"
		}
	}
}

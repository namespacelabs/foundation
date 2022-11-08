resourceClasses: {
	"Database": {
		intent: {
			type:   "library.database.redis.DatabaseIntent"
			source: "./api.proto"
		}
		produces: {
			type:   "library.database.redis.DatabaseInstance"
			source: "./api.proto"
		}
	}
}

resourceClasses: {
	"Database": {
		intent: {
			type:   "library.storage.redis.DatabaseIntent"
			source: "./api.proto"
		}
		produces: {
			type:   "library.storage.redis.DatabaseInstance"
			source: "./api.proto"
		}
	}
}

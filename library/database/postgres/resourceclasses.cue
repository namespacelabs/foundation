resourceClasses: {
	"Database": {
		intent: {
			type:   "library.database.postgres.DatabaseIntent"
			source: "./api.proto"
		}
		produces: {
			type:   "library.database.postgres.DatabaseInstance"
			source: "./api.proto"
		}
	}
}

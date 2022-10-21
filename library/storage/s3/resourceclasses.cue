resourceClasses: {
	"Bucket": {
		intent: {
			type:   "library.storage.s3.BucketIntent"
			source: "./api.proto"
		}
		produces: {
			type:   "library.storage.s3.BucketInstance"
			source: "./api.proto"
		}
	}
}

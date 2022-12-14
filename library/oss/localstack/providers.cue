providers: {
	"namespacelabs.dev/foundation/library/storage/s3:Bucket": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/localstack/prepare"

		resources: {
			server: {
				class:  "namespacelabs.dev/foundation/library/runtime:Server"
				intent: "namespacelabs.dev/foundation/library/oss/localstack/server"
			}
		}
	}
}

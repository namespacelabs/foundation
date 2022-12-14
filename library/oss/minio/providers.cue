providers: {
	"namespacelabs.dev/foundation/library/storage/s3:Bucket": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/minio/prepare"

		resources: {
			// Adds the server to the stack
			server: {
				class:  "namespacelabs.dev/foundation/library/runtime:Server"
				intent: "namespacelabs.dev/foundation/library/oss/minio/server"
			}
			// Mounts the MinIO user as a secret
			user: {
				class:  "namespacelabs.dev/foundation/library/runtime:Secret"
				intent: "namespacelabs.dev/foundation/library/oss/minio/server:user"
			}
			// Mounts the MinIO password as a secret
			password: {
				class:  "namespacelabs.dev/foundation/library/runtime:Secret"
				intent: "namespacelabs.dev/foundation/library/oss/minio/server:password"
			}
		}
	}
}

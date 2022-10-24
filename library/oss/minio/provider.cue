providers: {
	"namespacelabs.dev/foundation/library/storage/s3:Bucket": {
		initializedWith: imageFrom: binary: "namespacelabs.dev/foundation/library/oss/minio/prepare"

		resources: {
			// Adds the server to the stack
			minioServer: {
				class: "namespacelabs.dev/foundation/library/runtime:Server"
				intent: package_name: "namespacelabs.dev/foundation/library/oss/minio/server"
			}
			// Mounts the MinIO user as a secret
			minioUser: {
				class: "namespacelabs.dev/foundation/library/runtime:Secret"
				intent: ref: "namespacelabs.dev/foundation/library/oss/minio/server:user"
			}
			// Mounts the MinIO password as a secret
			minioPassword: {
				class: "namespacelabs.dev/foundation/library/runtime:Secret"
				intent: ref: "namespacelabs.dev/foundation/library/oss/minio/server:password"
			}
		}
	}
}

// This is a Namespace definition file.
// You can find a full syntax reference at https://namespace.so/docs/syntax-reference?utm_source=examples
server: {
	name: "withminiobucket"

	integration: "go"

	// Through adding a resource here, Namespace will
	// 1) instatiate an S3 Bucket using MinIO
	// 2) inject the configuration of the bucket (e.g. endpoint, access keys) into the resource config of our Go server
	resources: {
		dataBucket: {
			class:    "namespacelabs.dev/foundation/library/storage/s3:Bucket"
			provider: "namespacelabs.dev/foundation/library/oss/minio"

			intent: {
				bucketName: "testbucket"
			}
		}
	}

	env: {
		// Injects the bucket instance fields into environment variables.
		// Alternatively, could be read from /namespace/config/resources.json.
		// See also https://github.com/namespacelabs/foundation/tree/main/framework/resources/parsing.go
		S3_ACCESS_KEY_ID: fromResourceField: {
			resource: ":dataBucket"
			fieldRef: "accessKey"
		}

		S3_SECRET_ACCESS_KEY: fromResourceField: {
			resource: ":dataBucket"
			fieldRef: "secretAccessKey"
		}

		S3_BUCKET_NAME: fromResourceField: {
			resource: ":dataBucket"
			fieldRef: "bucketName"
		}

		S3_ENDPOINT_URL: fromResourceField: {
			resource: ":dataBucket"
			fieldRef: "url"
		}
	}
}

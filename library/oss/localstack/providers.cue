providers: {
	"namespacelabs.dev/foundation/library/storage/s3:Bucket": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/localstack/prepare/bucket"

		inputs: {
			cluster: {
				class:   "namespacelabs.dev/foundation/library/oss/localstack:Cluster"
				default: ":colocated"
			}
		}
	}
	"namespacelabs.dev/foundation/library/oss/localstack:Cluster": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/localstack/prepare/cluster"

		intent: {
			type:   "library.oss.localstack.ServerIntent"
			source: "./types.proto"
		}

		resourcesFrom: "namespacelabs.dev/foundation/library/oss/localstack/prepare/serverinstance"

		availableClasses: [
			"namespacelabs.dev/foundation/library/runtime:Server",
		]

		availablePackages: [
			"namespacelabs.dev/foundation/library/oss/localstack/server",
		]
	}
}

import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	awsclient "namespacelabs.dev/foundation/universe/aws/client"
	"namespacelabs.dev/foundation/universe/storage/minio/creds"
)

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	instantiate: {
		clientFactory: awsclient.#Exports.ClientFactory
		minioCreds:    creds.#Exports.Creds
	}

	provides: {
		Bucket: {
			input: $typesProto.types.BucketArgs
			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/universe/aws/s3"
					type:    "*Bucket"
				}
			}
		}
	}

	on: {
		prepare: {
			invokeBinary: {
				binary: "namespacelabs.dev/foundation/universe/storage/s3/internal/prepare"
			}
			requires: [
				"namespacelabs.dev/foundation/universe/development/localstack",
				"namespacelabs.dev/foundation/universe/storage/minio/server",
				"namespacelabs.dev/foundation/universe/storage/s3/internal/managebuckets",
			]
		}
	}
}

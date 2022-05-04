import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
	awsclient "namespacelabs.dev/foundation/universe/aws/client"
)

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	instantiate: {
		clientFactory:  awsclient.#Exports.ClientFactory
		readinessCheck: core.#Exports.ReadinessCheck
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
		}
	}
}

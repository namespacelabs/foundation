import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/aws/client"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	instantiate: {
		clientFactory:  client.#Exports.ClientFactory
		readinessCheck: core.#Exports.ReadinessCheck
	}

	provides: {
		Bucket: {
			input: $providerProto.types.BucketConfig
			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/universe/aws/s3"
					type:    "*Bucket"
				}
			}
		}
	}
}

$env: inputs.#Environment

configure: fn.#Configure & {
	// The internal/configure package gathers and provides invocation arguments into the init binary.
	with: binary: "namespacelabs.dev/foundation/universe/aws/s3/internal/configure"

	// Make sure the provided S3 bucket exists.
	init: [{
		binary: "namespacelabs.dev/foundation/universe/aws/s3/internal/managebuckets/init"
	}]
}

import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/secrets"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	instantiate: {
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

$localstackServer: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/universe/development/localstack"
}

configure: fn.#Configure & {
	// The internal/configure package gathers and provides invocation arguments into the `init` binary below.
	with: binary: "namespacelabs.dev/foundation/universe/development/localstack/s3/internal/configure"

	// Make sure the provided S3 bucket exists.
	init: [{
		binary: "namespacelabs.dev/foundation/universe/development/localstack/s3/internal/managebuckets/init"
	}]

	stack: {
		if $env.purpose == "DEVELOPMENT" || $env.purpose == "TESTING" {
			append: [$localstackServer]
		}
	}
	startup: {
		if $env.purpose == "DEVELOPMENT" || $env.purpose == "TESTING" {
			args: {
				localstack_endpoint: "http://\($localstackServer.$addressMap.api)"
			}
		}
	}
}

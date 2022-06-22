import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/universe/aws/client"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	instantiate: {
		clientFactory: client.#Exports.ClientFactory
	}

	provides: {
		Client: {
			input: $providerProto.types.ClientArgs
			availableIn: {
				go: {
					package: "github.com/aws/aws-sdk-go-v2/service/ecr"
					type:    "*Client"
				}
			}
		}
	}

	on: {
		prepare: {
			invokeInternal: "namespacelabs.dev/foundation/universe/aws/eks.DescribeCluster"
		}
	}
}

$env: inputs.#Environment

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/aws/ecr/configure"
}

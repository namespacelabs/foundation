import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"

	instantiate: {
		"interceptors": interceptors.#Exports.InterceptorRegistration
	}

	provides: {
		Deadlines: {
			input: $providerProto.types.Deadline
			availableIn: {
				go: {
					type:    "*DeadlineRegistration"
				}
			}
		}
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		InterceptorRegistration: {
			input: $providerProto.types.InterceptorRegistration

			availableIn: {
				go: type: "Registration"
			}
		}
	}
}

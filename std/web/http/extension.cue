import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		Backend: {
			input: $providerProto.types.Backend
			availableIn: {
				web: {}
			}
		}
	}

	on: {
		prepare: {
			invokeBinary: {
				binary: "namespacelabs.dev/foundation/std/web/http/configure"
			}
		}
	}
}

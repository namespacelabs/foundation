import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	framework: "WEB"

	provides: {
		Backend: {
			input: $providerProto.types.Backend
			availableIn: {
				web: {}
			}
		}
	}
}

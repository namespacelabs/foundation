import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		Data: {
			input: $providerProto.types.Input

			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/internal/testdata/counter/data"
					type:    "*Data"
				}
			}
		}
	}
}

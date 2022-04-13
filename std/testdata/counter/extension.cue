import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/testdata/counter/data"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		Counter: {
			input: $providerProto.types.Input

			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/std/testdata/counter"
					type:    "*Counter"
				}
			}

			// Artificial instantiate used for e2e testing of scoped instantiation.
			instantiate: {
				"data": data.#Exports.Data
			}
		}
	}
}

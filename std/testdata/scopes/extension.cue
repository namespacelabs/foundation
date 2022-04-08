import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/testdata/scopes/data"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		ScopedData: {
			input: $providerProto.types.Input

			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/std/testdata/scopes"
					type:    "*ScopedData"
				}
			}
            instantiate: {
                "data": data.#Exports.Data
            }
		}
	}
}
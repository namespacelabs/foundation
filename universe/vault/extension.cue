import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		ClientHandle: {
			input: $providerProto.types.VaultClientArgs
			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/universe/vault"
					type:    "*ClientHandle"
				}
			}
		}
	}
}

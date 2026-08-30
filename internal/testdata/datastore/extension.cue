import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/core"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	instantiate: {
		readinessCheck: core.#Exports.ReadinessCheck
	}

	provides: {
		Database: {
			input: $providerProto.types.Database

			availableIn: {
				go: type: "*DB"
			}
		}
	}
}

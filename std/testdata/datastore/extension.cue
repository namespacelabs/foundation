import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/secrets"
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
			instantiate: {
				cert: secrets.#Exports.Secret & {
					name: "cert"
				}
				gen: secrets.#Exports.Secret & {
					name: "gen"
					generate: {
						randomByteCount: 32
					}
				}
				keygen: secrets.#Exports.Secret & {
					name: "keygen"
					initializeWith: {
						binary: "namespacelabs.dev/foundation/std/testdata/datastore/keygen"
					}
				}
			}
		}
	}
}

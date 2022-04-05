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
		cert: secrets.#Exports.Secret & {
			with: {
				name: "cert"
			}
		}
		gen: secrets.#Exports.Secret & {
			with: {
				name: "gen"
				generate: {
					randomByteCount: 32
				}
			}
		}
		keygen: secrets.#Exports.Secret & {
			with: {
				name: "keygen"
				initializeWith: {
					binary: "namespacelabs.dev/foundation/std/testdata/datastore/keygen"
				}
			}
		}
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

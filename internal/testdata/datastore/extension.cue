import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/std/secrets"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
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
				binary: "namespacelabs.dev/foundation/internal/testdata/datastore/keygen"
				// The binary path is the battle-tested path;
				// experimentalFunction is an exploration on a new method to
				// invoke provisioning tools.
				//
				// experimentalFunction: "namespacelabs.dev/foundation/internal/testdata/datastore/denokeygen"
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

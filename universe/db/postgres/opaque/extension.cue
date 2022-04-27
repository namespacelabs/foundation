import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/opaque/creds"
)

$providerProto: inputs.#Proto & {
	sources: [
		"provider.proto",
		"../database.proto",
	]
}

extension: fn.#Extension & {
	instantiate: {
		wire: postgres.#Exports.WireDatabase
	}

	provides: {
		Database: {
			input: $providerProto.types.Database

			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/universe/db/postgres"
					type:    "*DB"
				}
			}
			instantiate: {
				"creds": creds.#Exports.Creds
			}
		}
	}
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/db/postgres/opaque/tool"
}

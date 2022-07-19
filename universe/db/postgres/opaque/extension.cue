import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/base"
	"namespacelabs.dev/foundation/universe/db/postgres/opaque/creds"
)

$providerProto: inputs.#Proto & {
	sources: [
		"provider.proto",
		"../database.proto",
	]
}

// This extension provides simple compatibility with an existing database.
// Usage is discouraged as Namespace won't be able to manage the database (e.g. environments are not isolated).
extension: fn.#Extension & {
	instantiate: {
		wire: base.#Exports.WireDatabase
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

	init: setup: {
		binary: "namespacelabs.dev/foundation/universe/db/postgres/internal/init"
	}
}

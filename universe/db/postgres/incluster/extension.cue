import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/base"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/gencreds"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
  // resources: {
	// 	logins: {
	// 		class:    "namespacelabs.dev/foundation/library/database/postgres:Database"
	// 		provider: "namespacelabs.dev/foundation/library/oss/postgres"

	// 		intent: {
	// 			name: "logins"
	// 			schema: ["schema.sql"]
	// 		}

	// 		resources: {
	// 			"cluster": "namespacelabs.dev/foundation/library/oss/postgres:colocated"
	// 		}
	// 	}
	// }

	instantiate: {
		// TODO: Move creds instantiation into provides when the server supports multiple users
		creds: gencreds.#Exports.Creds
		wire:  base.#Exports.WireDatabase
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
		}
	}
}

$server: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/universe/db/postgres/server"
}

configure: fn.#Configure & {
	stack: {
		append: [$server]
	}

	with: binary: "namespacelabs.dev/foundation/universe/db/postgres/incluster/tool"

	init: setup: {
		binary: "namespacelabs.dev/foundation/universe/db/postgres/internal/init"
	}
}

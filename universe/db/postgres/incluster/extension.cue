import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/base"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	instantiate: {
		// TODO: Move creds instantiation into provides when the server supports multiple users
		"creds": creds.#Exports.Creds
		wire:    base.#Exports.WireDatabase
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
}

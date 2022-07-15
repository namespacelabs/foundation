import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/aws/client"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/base"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	instantiate: {
		clientFactory: client.#Exports.ClientFactory
		// TODO: Move creds instantiation into provides when incluster server supports multiple creds
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

	on: {
		prepare: {
			invokeBinary: {
				binary: "namespacelabs.dev/foundation/universe/db/postgres/rds/internal/prepare"
			}
			requires: [
				"namespacelabs.dev/foundation/universe/db/postgres/incluster/tool",
				"namespacelabs.dev/foundation/universe/db/postgres/internal/init",
				"namespacelabs.dev/foundation/universe/db/postgres/server",
			]
		}
	}
}

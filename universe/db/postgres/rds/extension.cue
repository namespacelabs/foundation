import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/aws/client"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/base"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/gencreds"
	"namespacelabs.dev/foundation/std/core/info"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

// Experimental! This extension is still under active development.
extension: fn.#Extension & {
	instantiate: {
		clientFactory: client.#Exports.ClientFactory
		// TODO: Move creds instantiation into provides when incluster server supports multiple creds
		creds:      gencreds.#Exports.Creds
		wire:       base.#Exports.WireDatabase
		serverInfo: info.#Exports.ServerInfo
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
				imageFrom: binary: "namespacelabs.dev/foundation/universe/db/postgres/rds/prepare"
			}
			requires: [
				"namespacelabs.dev/foundation/universe/db/postgres/internal/init",
				"namespacelabs.dev/foundation/universe/db/postgres/rds/init",
				"namespacelabs.dev/foundation/universe/db/postgres/rds/internal/server",
			]
		}
	}
}

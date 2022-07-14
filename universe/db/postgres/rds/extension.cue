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

// This extension uses RDS for all environments.
// We recommend to use namespacelabs.dev/foundation/universe/db/postgres instead.
extension: fn.#Extension & {
	instantiate: {
		clientFactory:  client.#Exports.ClientFactory
		// TODO: Move creds instantiation into provides
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

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/db/postgres/rds/tool"
}
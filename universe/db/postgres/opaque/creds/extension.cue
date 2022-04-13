import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/secrets"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		Creds: {
			input: $providerProto.types.CredsRequest

			availableIn: {
				go: type: "*Creds"
			}
			instantiate: {
				user: secrets.#Exports.Secret & {
					name: "postgres-user-file"
				}
				password: secrets.#Exports.Secret & {
					name: "postgres-password-file"
				}
			}
		}
	}
}

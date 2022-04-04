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
	instantiate: {
		user: secrets.#Exports.Secret & {
			with: {
				name: "postgres_user_file"
			}
		}
		password: secrets.#Exports.Secret & {
			with: {
				name: "postgres_password_file"
			}
		}
	}

	provides: {
		Creds: {
			input: $providerProto.types.CredsRequest

			availableIn: {
				go: type: "*Creds"
			}
		}
	}
}

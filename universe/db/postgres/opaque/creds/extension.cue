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
				name: "postgres-user-file"
				provision: ["PROVISION_INLINE", "PROVISION_AS_FILE"]
			}
		}
		password: secrets.#Exports.Secret & {
			with: {
				name: "postgres-password-file"
				provision: ["PROVISION_INLINE", "PROVISION_AS_FILE"]
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

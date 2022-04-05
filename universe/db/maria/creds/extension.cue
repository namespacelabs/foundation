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
		password: secrets.#Exports.Secret & {
			with: {
				name: "mariadb-password-file"
				provision: ["PROVISION_INLINE", "PROVISION_AS_FILE"]
				generate: {
					randomByteCount: 32
				}
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

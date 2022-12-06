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
			name: "postgres-password-file"
			generate: {
				uniqueId:        "2cmo5ocirf1k00h7idj0"
				randomByteCount: 32
				format:          "FORMAT_BASE32"
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

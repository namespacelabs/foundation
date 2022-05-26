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
			name:                      "root-user"
			experimentalMountAsEnvVar: "MINIO_ROOT_USER"
			generate: {
				randomByteCount: 8
				format:          "FORMAT_BASE32"
			}
		}
		root_password: secrets.#Exports.Secret & {
			name:                      "root-password"
			experimentalMountAsEnvVar: "MINIO_ROOT_PASSWORD"
			generate: {
				randomByteCount: 16
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

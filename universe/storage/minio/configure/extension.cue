import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/secrets"
)

extension: fn.#Extension & {
	instantiate: {
		"access_token": secrets.#Exports.Secret & {
			name:     "access_token"
		}
		"secret_key": secrets.#Exports.Secret & {
			name:     "secret_key"
		}
	}
}

configure: fn.#Configure & {
    with: binary: "namespacelabs.dev/foundation/universe/storage/minio/internal/prepare"
	startup: {
		args: [
			"server",
			"/tmp", // TODO mount storage
			"--address=:9000",
			"--console-address=:9001",
		]
		env: {
			// "MINIO_ROOT_USER":     "access_key_value"
			// "MINIO_ROOT_PASSWORD": "secret_key_value"
		}
	}
}

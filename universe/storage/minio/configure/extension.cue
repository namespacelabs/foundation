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
	// Provide the rest of the configuration (e.g. requred secrets) here:
	with: binary: "namespacelabs.dev/foundation/universe/storage/minio/internal/provision"

	startup: {
		args: [
			"server",
			"/tmp",
			"--address=:9000",
			"--console-address=:9001",
		]
	}
}

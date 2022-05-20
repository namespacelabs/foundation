import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension

configure: fn.#Configure & {
	startup: {
		args: [
			"server",
			"/tmp", // TODO mount storage
			"--address=:9000",
			"--console-address=:9001",
		]
		env: {
			"MINIO_ROOT_USER":     "access_key_value"
			"MINIO_ROOT_PASSWORD": "secret_key_value"
		}
	}
}

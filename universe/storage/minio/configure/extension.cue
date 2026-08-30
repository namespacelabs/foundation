import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	import: [
		"namespacelabs.dev/foundation/universe/storage/minio/creds",
	]
}

configure: fn.#Configure & {
	startup: {
		args: [
			"server",
			"/tmp",
			"--address=:9000",
			"--console-address=:9001",
		]

		env: {
			// Disable update checking as self-update will never be used.
			MINIO_UPDATE: "off"
		}
	}
}

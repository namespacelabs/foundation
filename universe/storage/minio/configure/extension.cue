import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/secrets"
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
	}
}

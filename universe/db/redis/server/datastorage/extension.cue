import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "redis-data"
		byteCount:    "10GiB"
		mountPath:    "/data"
	}
}

configure: fn.#Configure & {
	startup: {
		args: ["--save", "60", "1"]
	}
}

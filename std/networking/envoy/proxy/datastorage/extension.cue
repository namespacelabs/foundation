import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "envoy-data"
		byteCount:    "10MiB"
		mountPath:    "/config"
	}
}

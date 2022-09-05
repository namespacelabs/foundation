import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "ns-orchestration-data"
		byteCount:    "10GiB"
		mountPath:    "/namespace/orchestration/data"
	}
}

configure: fn.#Configure & {
	startup: {
		env: {
			"NSDATA": "/namespace/orchestration/data"
			"HOME":   "/namespace/orchestration/data/tmphome" // TODO Move to empty dir.
		}
	}
}

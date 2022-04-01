import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "postgres-data"
		byteCount:    "10GiB"
		mountPath:    "/postgres/data"
	}
}

configure: fn.#Configure & {
	startup: {
		env: {
			// PGDATA may not be a mount point but only a subdirectory.
			"PGDATA": "/postgres/data/pgdata"
		}
	}
}

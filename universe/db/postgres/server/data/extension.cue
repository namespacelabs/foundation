import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "postgres-data"
		byteCount:    "100GiB" // XXX this was originally 10GiB, bumped to workaround the unit bug.
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

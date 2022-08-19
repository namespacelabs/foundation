import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "rds-postgres-data"
		byteCount:    "1GiB" // Test/dev only
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

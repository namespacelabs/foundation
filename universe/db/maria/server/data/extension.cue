import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "mariadb-data"
		byteCount:    "10GiB"
		mountPath:    "/mariadb/data"
	}
}

configure: fn.#Configure & {
	startup: {
		args: {
			v: "/mariadb/data/datadir:/var/lib/mysql"
		}
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "mariadb-data"
		byteCount:    "10GiB"
		mountPath:    "/var/lib/mysql"
	}
}

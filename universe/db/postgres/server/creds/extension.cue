import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	import: [
		"namespacelabs.dev/foundation/universe/db/postgres/creds",
	]
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/db/postgres/server/creds/tool"
}

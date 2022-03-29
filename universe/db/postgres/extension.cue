import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/db/postgres/tool"

	init: [{
		binary: "namespacelabs.dev/foundation/universe/db/postgres/init"
	}]
}

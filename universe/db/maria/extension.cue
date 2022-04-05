import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/db/maria/tool"

	init: [{
		binary: "namespacelabs.dev/foundation/universe/db/maria/init"
	}]
}

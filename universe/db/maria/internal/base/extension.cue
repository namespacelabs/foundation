import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension

configure: fn.#Configure & {
	init: [{
		binary: "namespacelabs.dev/foundation/universe/db/maria/internal/init"
	}]
}

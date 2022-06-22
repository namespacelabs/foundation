import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/core"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	instantiate: {
		debugHandler: core.#Exports.DebugHandler
	}
}

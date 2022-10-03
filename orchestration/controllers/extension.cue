import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/core"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	// We don't really need it, but https://github.com/namespacelabs/foundation/issues/717
	instantiate: {
		ready: core.#Exports.ReadinessCheck
	}
}

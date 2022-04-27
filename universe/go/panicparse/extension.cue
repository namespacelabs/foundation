import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/go/core"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"

	instantiate: {
		debugHandler: core.#Exports.DebugHandler
	}
}

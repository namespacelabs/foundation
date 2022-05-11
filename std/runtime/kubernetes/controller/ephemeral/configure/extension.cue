import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

// TODO #349 remove empty extension.
extension: fn.#Extension

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/std/runtime/kubernetes/controller/ephemeral/tool"
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

// We use "hasInitializerIn" to indicate that it is a Web extension.
// Codegen for initializers and providers is not supported in Web extensions.
// TODO: figure out a better semantics for Web nodes. This is not the final state.
extension: fn.#Extension & {
	hasInitializerIn: "WEB"
}

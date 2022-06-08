import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	// Needed so Foundation recognizes this node as a Node.js package.
	hasInitializerIn: "NODEJS"
}

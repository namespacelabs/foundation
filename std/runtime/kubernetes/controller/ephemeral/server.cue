import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "vvmln00pguusl9idcv9g"
	name: "ephemeralcontroller"

	binary: "namespacelabs.dev/foundation/std/runtime/kubernetes/controller/ephemeral/img"

	import: [
		"namespacelabs.dev/foundation/std/runtime/kubernetes/controller/ephemeral/configure",
	]
}

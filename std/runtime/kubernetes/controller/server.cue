import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "vvmln00pguusl9idcv9g"
	name: "kubernetescontroller"

	binary: "namespacelabs.dev/foundation/std/runtime/kubernetes/controller/img"

	import: [
		"namespacelabs.dev/foundation/std/runtime/kubernetes/controller/configure",
	]
}

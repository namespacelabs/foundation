import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "vvmln00pguusl9idcv9g"
	name: "fn-controller"

	binary: "namespacelabs.dev/foundation/std/runtime/kubernetes/controller/img"
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/std/runtime/kubernetes/controller/tool"
}

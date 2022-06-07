import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
)

extension: fn.#Extension & {
}

$envoyProxy: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/networking/envoy/proxy"
}

configure: fn.#Configure & {
	stack: {
		append: [$envoyProxy]
	}
	sidecar: [{
		binary: "namespacelabs.dev/foundation/std/networking/envoy/controller"
	}]
}

import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
)

extension: fn.#Extension & {
	requirePersistentStorage: {
		persistentId: "envoy-data"
		byteCount:    "10MiB"
		mountPath:    "/config"
	}
}

$envoyProxy: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/networking/envoy/proxy"
}

configure: fn.#Configure & {
	stack: {
		append: [$envoyProxy]
	}
	init: [{
		binary: "namespacelabs.dev/foundation/std/networking/envoy/init"
	}]
}

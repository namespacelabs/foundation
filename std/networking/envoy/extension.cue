import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
)

extension: fn.#Extension & {
}

configure: fn.#Configure & {
	sidecar: [{
		binary: "namespacelabs.dev/foundation/std/networking/envoy/proxy"
	}]
}

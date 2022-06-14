import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

// This extension represents a std/networking/gateway-based transcoding setup.
extension: fn.#Extension & {}

$gateway: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/networking/gateway/server"
}

configure: fn.#Configure & {
	stack: append: [$gateway]
	with: {
		binary: "namespacelabs.dev/foundation/std/grpc/httptranscoding/configure"
		inject: ["schema.ComputedNaming"]
	}
}

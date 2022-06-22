import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension & {
	import: [
		"namespacelabs.dev/foundation/std/core",
		"namespacelabs.dev/foundation/std/go/grpc/metrics",
	]
}

$inputs: {
	serverPort: inputs.#Port & {
		name: "server-port"
	}
}

configure: fn.#Configure & {
	startup: {
		args: {
			listen_hostname: "0.0.0.0"
			port:            "\($inputs.serverPort.port)"
		}
	}
}

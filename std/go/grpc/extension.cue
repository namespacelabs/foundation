import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension & {
	import: [
		"namespacelabs.dev/foundation/std/go/core",
		"namespacelabs.dev/foundation/std/go/grpc/metrics",
		"namespacelabs.dev/foundation/std/monitoring/tracing",
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
			listen_hostname: "0.0.0.0" // docker_proxy needs to be able to connect.
			port:            "\($inputs.serverPort.port)"
		}
	}
}

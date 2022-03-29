import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension & {
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

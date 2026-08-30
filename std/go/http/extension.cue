import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$inputs: {
	httpPort: inputs.#Port & {
		name: "http-port"
	}
}

extension: fn.#Extension & {}

configure: fn.#Configure & {
	startup: {
		args: {
			grpcserver_http_port: "\($inputs.httpPort.port)"
		}
	}
}

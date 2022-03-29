import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$inputs: {
	gatewayPort: inputs.#Port & {
		name: "grpc-gateway-port"
	}
}

extension: fn.#Extension & {}

configure: fn.#Configure & {
	startup: {
		args: {
			gateway_port: "\($inputs.gatewayPort.port)"
		}
	}
}

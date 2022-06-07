import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		GrpcRegistrar: {
			input: $providerProto.types.NoArgs
			availableIn: {
				nodejs: {
					import: "registrar"
					type:   "GrpcRegistrar"
				}
			}
		}
		GrpcInterceptorRegistrar: {
			input: $providerProto.types.NoArgs
			availableIn: {
				nodejs: {
					import: "interceptor"
					type:   "GrpcInterceptorRegistrar"
				}
			}
		}
	}
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

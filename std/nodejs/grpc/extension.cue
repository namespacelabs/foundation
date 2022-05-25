import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/nodejs/grpcgen"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	import: [
		"namespacelabs.dev/foundation/std/nodejs/grpcgen",
	]

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

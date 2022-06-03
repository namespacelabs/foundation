import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		HttpServer: {
			input: $providerProto.types.NoArgs
			availableIn: {
				nodejs: {
					import: "httpserver"
					type:   "Promise<HttpServer>"
				}
			}
		}
	}
}

$inputs: {
	httpPort: inputs.#Port & {
		name: "http-port"
	}
}

configure: fn.#Configure & {
	startup: {
		args: {
			http_port: "\($inputs.httpPort.port)"
		}
	}
}

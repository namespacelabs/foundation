import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$inputs: {
	httpPort: inputs.#Port & {
		name: "http-port"
	}
}
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

configure: fn.#Configure & {
	startup: {
		args: {
			http_port: "\($inputs.httpPort.port)"
		}
	}
}

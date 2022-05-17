import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

// TODO: consolidate with std/go/http extension.

$inputs: {
	httpPort: inputs.#Port & {
		name: "http-port"
	}
}

extension: fn.#Extension & {}

configure: fn.#Configure & {
	startup: {
		args: {
			http_port: "\($inputs.httpPort.port)"
		}
	}
}

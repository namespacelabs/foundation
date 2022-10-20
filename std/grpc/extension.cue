import (
	"path"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc"
)

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	provides: {
		Backend: {
			input: $providerProto.types.Backend
			availableIn: {
				go: {} // Computed at runtime.
			}
		}

		Conn: {
			input: $providerProto.types.Backend
			availableIn: {
				go: {
					package: "google.golang.org/grpc"
					type:    "*ClientConn"
				}
			}
		}
	}
}

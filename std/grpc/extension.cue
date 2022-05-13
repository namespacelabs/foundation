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
				nodejs: {} // Computed at runtime.
			}
		}

		Conn: {
			input: $providerProto.types.Backend
			availableIn: {
				go: {
					package: "google.golang.org/grpc"
					type:    "*ClientConn"
				}
				// nodejs doesn't have a type for a connection. Re-using a channel happens automatically:
				// https://github.com/grpc/grpc-node/issues/359
			}
		}
	}
}

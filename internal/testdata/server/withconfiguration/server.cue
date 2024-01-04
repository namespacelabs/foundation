import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

server: fn.#Server & {
	id:        "brc373trnbon6ecla45g"
	name:      "withconfiguration"
	framework: "GO"

	listeners: {
		"mtls": {
			protocol: "grpc"
			port: {
				containerPort: 12345
			}
		}
		"second": {
			port: {
				containerPort: 12346
			}
		}
	}

	import: [
		"namespacelabs.dev/foundation/internal/testdata/service/grpclistener",
		"namespacelabs.dev/foundation/internal/testdata/service/rawlistener",
		"namespacelabs.dev/foundation/std/grpc/logging",
	]
}

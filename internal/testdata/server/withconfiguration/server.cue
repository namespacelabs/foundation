import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

server: fn.#Server & {
	id:        "brc373trnbon6ecla45g"
	name:      "withconfiguration"
	framework: "GO"

	staticPorts: {
		"server-port-mtls": {
			containerPort: 12345
		}
	}

	import: [
		"namespacelabs.dev/foundation/internal/testdata/service/simplewithconfiguration",
		"namespacelabs.dev/foundation/std/grpc/logging",
	]
}

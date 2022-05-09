import (
	"namespacelabs.dev/foundation/std/fn"
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/multidb",
)

server: fn.#Server & {
	id:        "inar7bsnjhsfptlp50r0"
	name:      "multidbserver"
	framework: "GO_GRPC"

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/multidb",
	]
}

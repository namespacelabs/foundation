import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "inar7bsnjhsfptlp50r0"
	name:      "multidbserver"
	framework: "GO"
	testonly:  true

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/multidb",
	]
}

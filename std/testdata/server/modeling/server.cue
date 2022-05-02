import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "m9v09613qsdlddj3a8i0"
	name:      "modelingserver" // test server that allows testing data modeling of foundation
	framework: "GO_GRPC"

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/multicounter",
	]
}

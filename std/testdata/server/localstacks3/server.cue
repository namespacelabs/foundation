import (
	"namespacelabs.dev/foundation/std/fn"
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/localstacks3",
)

server: fn.#Server & {
	id:        "rn5q3mcug1dnkbtue3cg"
	name:      "localstacks3"
	framework: "GO_GRPC"

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/localstacks3",
	]
}

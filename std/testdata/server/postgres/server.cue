import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

server: fn.#Server & {
	id:        "q311n9u9uvirr2i42ms0"
	name:      "postgresserver"
	framework: "GO_GRPC"

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/list",
	]
}

import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "q311n9u9uvirr2i42ms0"
	name:      "postgresserver"
	framework: "GO"
	testonly:  true

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/list",
	]
}

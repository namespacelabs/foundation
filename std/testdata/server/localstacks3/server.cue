import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "rn5q3mcug1dnkbtue3cg"
	name:      "localstacks3"
	framework: "GO"

	import: [
		"namespacelabs.dev/foundation/std/go/grpc/gateway",
		"namespacelabs.dev/foundation/std/testdata/service/localstacks3",
	]
}

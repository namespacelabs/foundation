import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "q311n9u9uvirr2i42ms0"
	name:      "postgresserver"
	framework: "GO"

	import: [
		"namespacelabs.dev/foundation/std/testdata/service/list",
	]
}

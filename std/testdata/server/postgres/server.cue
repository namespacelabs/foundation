import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "q311n9u9uvirr2i42ms0"
	name:      "postgresdemoserver"
	framework: "GO"
	testonly:  true

	import: [
		"namespacelabs.dev/foundation/std/testdata/service/list",
	]
}

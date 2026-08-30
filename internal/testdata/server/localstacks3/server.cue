import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "rn5q3mcug1dnkbtue3cg"
	name:      "localstacks3"
	framework: "GO"
	testonly:  true

	import: [
		"namespacelabs.dev/foundation/internal/testdata/service/localstacks3",
	]
}

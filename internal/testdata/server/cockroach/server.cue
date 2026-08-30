import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "7lb5mgu97tecmdk2muug"
	name:      "cockroachdemoserver"
	framework: "GO"
	testonly:  true

	import: [
		"namespacelabs.dev/foundation/internal/testdata/service/roachlist",
	]
}

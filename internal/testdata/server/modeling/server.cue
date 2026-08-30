import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "m9v09613qsdlddj3a8i0"
	name:      "modelingserver" // test server that allows testing data modeling of foundation
	framework: "GO"
	testonly:  true

	import: [
		"namespacelabs.dev/foundation/internal/testdata/service/count",
	]
}

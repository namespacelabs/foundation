import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "inar7bsnjhsfptlp50r0"
	name:      "multidbserver"
	framework: "GO"

	import: [
		"namespacelabs.dev/foundation/std/testdata/service/multidb",
	]
}

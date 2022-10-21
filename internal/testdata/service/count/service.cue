import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/internal/testdata/counter"
)

$proto: inputs.#Proto & {
	source: "../proto/count.proto"
}

service: fn.#Service & {
	framework: "GO"

	instantiate: {
		one: counter.#Exports.Counter & {
			name: "one"
		}
		two: counter.#Exports.Counter & {
			name: "two"
		}
	}

	exportService: $proto.services.CountService
}

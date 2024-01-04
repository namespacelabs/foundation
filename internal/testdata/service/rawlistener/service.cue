import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
	"namespacelabs.dev/foundation/internal/testdata/counter"
)

service: fn.#Service & {
	framework:    "GO"
	listener:     "second"
	ingress:      "LOAD_BALANCER"
	exportedPort: 10000

	instantiate: {
		one: counter.#Exports.Counter & {
			name: "one"
		}
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

extension: fn.#Extension & {
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/aws/irsa/prepare"
}

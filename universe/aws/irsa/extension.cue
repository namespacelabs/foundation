import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

extension: fn.#Extension & {
}

$env: inputs.#Environment

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/aws/irsa/prepare"

	details: {
		"namespacelabs.dev/foundation/std/runtime/kubernetes": {
			ensureServiceAccount: true
		}

		"namespacelabs.dev/foundation/universe/aws/eks": {
			describeIfEks: true
		}
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
}

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

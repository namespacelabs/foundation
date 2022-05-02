import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/runtime/kubernetes"
)

extension: fn.#Extension & {
	instantiate: {
		k8s: kubernetes.#Exports.ServerExtension & {
			ensureServiceAccount: true
		}
	}

	on: {
		prepare: {
			invokeInternal: "namespacelabs.dev/foundation/universe/aws/eks.DescribeCluster"
		}
	}
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

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	instantiate: {
		"detector": tracing.#Exports.Detector & {
			name: "kubernetes"
		}
	}
}

configure: fn.#Configure & {
	startup: {
		env: {
			"TRACING_K8S_NAMESPACE": experimentalFromDownwardsFieldPath: "metadata.namespace"
			"TRACING_K8S_POD_NAME": experimentalFromDownwardsFieldPath:  "metadata.name"
			"TRACING_K8S_POD_UID": experimentalFromDownwardsFieldPath:   "metadata.uid"
			"TRACING_K8S_NODE_NAME": experimentalFromDownwardsFieldPath: "spec.nodeName"
		}
	}
}

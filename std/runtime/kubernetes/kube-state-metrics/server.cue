import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:        "c9kf8ejnhamknruls850"
	name:      "kube-state-metrics"

	binary: image: "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.5.0"
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/std/runtime/kubernetes/kube-state-metrics/configure"
}

import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "rvbjsgm9fbukqa75u2p0"
	name: "k8s-event-exporter"

	binary: image: "ghcr.io/opsgenie/kubernetes-event-exporter:v0.11"
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/universe/networking/k8s-event-exporter/configure"
}

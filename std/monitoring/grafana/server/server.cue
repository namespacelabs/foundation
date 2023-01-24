import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "4dt58k5hmmwh5sq878msdwpn74"
	name: "Grafana"

	binary: {
		image: "grafana/grafana-enterprise:9.3.2@sha256:b7e2a549ccbb51319c792559df720af1398db5a89a1405a159bdfa6bbad382fb"
	}

	service: "web": {
		label:         "Grafana"
		containerPort: 3000
		metadata: protocol: "http"
	}
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/std/monitoring/grafana/tool"
}

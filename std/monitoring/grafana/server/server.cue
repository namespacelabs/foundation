import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "4dt58k5hmmwh5sq878msdwpn74"
	name: "Grafana"

	binary: {
		image: "grafana/grafana-enterprise:8.2.3@sha256:436c264303bb2cc03ba91912ee2711f1bf029f12ba5a7ec1db1a6e1f8774d9c8"
	}

	import: [
		"namespacelabs.dev/foundation/std/monitoring/grafana/server/configure",
	]

	service: "web": {
		containerPort: 3000
		metadata: protocol: "http"
	}
}

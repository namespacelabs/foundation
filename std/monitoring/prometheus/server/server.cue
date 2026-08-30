import "namespacelabs.dev/foundation/std/fn"

server: fn.#OpaqueServer & {
	id:   "04ntvj9zen9ns6ha76mvfb9z4a"
	name: "prometheus"

	binary: {
		image: "prom/prometheus:v2.37.5@sha256:8fa63fdd8d48e12bc8cd5e84b3e39e8ebf3cbd3580fb2c6449167917aaf0f04e"
	}

	service: "prometheus": {
		label:         "Prometheus"
		containerPort: 9090
		metadata: {
			kind:     "prometheus.io/endpoint"
			protocol: "http"
		}
	}
}

configure: fn.#Configure & {
	with: {
		binary: "namespacelabs.dev/foundation/std/monitoring/prometheus/tool"
		args: {
			mode: "server"
		}
	}
}

import "namespacelabs.dev/foundation/std/fn"

server: fn.#OpaqueServer & {
	id:   "04ntvj9zen9ns6ha76mvfb9z4a"
	name: "Prometheus"

	binary: {
		image: "prom/prometheus:v2.31.1@sha256:a8779cfe553e0331e9046268e26c539fa39ecf90d59836d828163e65e8f4fa35"
	}

	service: "prometheus": {
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

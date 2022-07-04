import "namespacelabs.dev/foundation/std/fn"

server: fn.#OpaqueServer & {
	id:   "23crtq628x848gfajbfhv6btj8"
	name: "Jaeger"

	binary: {
		image: "jaegertracing/all-in-one:1.27@sha256:8d0bff43db3ce5c528cb6f957520511d263d7cceee012696e4afdc9087919bb9"
	}

	service: "collector": {
		containerPort: 14268
		metadata: {
			protocol: "thrift"
		}
		internal: true // Not used for development.
	}

	service: "web": {
		label: "Jaeger"
		containerPort: 16686
		metadata: {
			protocol: "http"
		}
	}
}

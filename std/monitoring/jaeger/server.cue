import "namespacelabs.dev/foundation/std/fn"

server: fn.#OpaqueServer & {
	id:   "23crtq628x848gfajbfhv6btj8"
	name: "Jaeger"

	binary: {
		image: "jaegertracing/all-in-one:1.42@sha256:7d32a4eddec7b9a6d8265d4320a224e06ab5fe0753869715e5852401e8e7d6eb"
	}

	service: "otel-grpc": {
		containerPort: 4317
		metadata: protocol: "grpc"
	}

	service: "collector": {
		containerPort: 14268
		metadata: protocol: "http"
	}

	service: "web": {
		label:         "Jaeger"
		containerPort: 16686
		metadata: protocol: "http"
	}
}

configure: {
	startup: {
		args: {
			"memory.max-traces": "2500000"
		}

		env: {
			COLLECTOR_OTLP_ENABLED: "true"
		}
	}
}

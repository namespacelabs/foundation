server: {
	name: "otel-collector"

	imageFrom: binary: "namespacelabs.dev/foundation/std/monitoring/otelcollector/trampoline"

	env: {
		// JAEGER_ENDPOINT: fromServiceEndpoint: "namespacelabs.dev/foundation/std/monitoring/jaeger:otel-grpc"
		HONEYCOMB_TEAM: fromSecret:           "namespacelabs.dev/foundation/universe/monitoring/honeycomb:xHoneycombTeam"
	}

	services: {
		"otel-grpc": {
			port: 4317
		}
		"otel-http": {
			port: 4318
		}
	}

	mounts: "/otel/conf": ephemeral: {}

	// requires: [
	// 	"namespacelabs.dev/foundation/std/monitoring/jaeger",
	// ]
}

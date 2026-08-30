server: {
	name: "otel-collector"

	imageFrom: binary: "namespacelabs.dev/foundation/std/monitoring/otelcollector/trampoline"

	env: {
		if $env.purpose == "DEVELOPMENT" || $env.purpose == "TESTING" {
			// We need at least one exporter.
			JAEGER_ENDPOINT: fromServiceEndpoint: "namespacelabs.dev/foundation/std/monitoring/jaeger:otel-grpc"
		}
		HONEYCOMB_TEAM: fromSecret: "namespacelabs.dev/foundation/universe/monitoring/honeycomb:xHoneycombTeam"
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

	if $env.purpose == "DEVELOPMENT" || $env.purpose == "TESTING" {
		requires: [
			"namespacelabs.dev/foundation/std/monitoring/jaeger",
		]
	}
}

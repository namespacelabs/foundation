binary: {
	name: "otelcol-trampoline"

	build_plan: {
		layer_build_plan: [
			{prebuilt: "otel/opentelemetry-collector-contrib:0.78.0"},
			{go_build: {
				rel_path:    "."
				binary_name: "trampoline"
				binary_only: true
			}},
		]
	}

	config: {
		command: ["/trampoline"]
	}
}

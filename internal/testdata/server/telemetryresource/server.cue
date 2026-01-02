import "namespacelabs.dev/foundation/std/fn"

server: fn.#Server & {
	id:        "telemetryresourcetest001"
	name:      "telemetryresourceserver"
	framework: "GO"

	resource: {
		"service.name": "custom-otel-service-name"
	}

	import: [
		"namespacelabs.dev/foundation/internal/testdata/service/telemetryinfo",
	]
}

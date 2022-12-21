import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"
	initializeBefore: ["namespacelabs.dev/foundation/std/monitoring/tracing"]

	instantiate: {
		openTelemetry: tracing.#Exports.Exporter & {
			name: "lightstep"
		}
	}
}

configure: fn.#Configure & {
	startup: {
		env: {
			// TODO: support optional secrets
			if $env.name == "prod-metal" {
				"MONITORING_LIGHTSTEP_ACCESS_TOKEN": fromSecret: ":lightstepAccessToken"
			}
		}
	}
}

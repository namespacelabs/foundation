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
			name: "honeycomb"
		}
	}
}

configure: fn.#Configure & {
	startup: {
		env: {
			// TODO: support optional secrets
			if $env.name == "dev" || $env.name == "staging" || $env.name == "prod" || $env.name == "prod-metal" {
				"MONITORING_HONEYCOMB_X_HONEYCOMB_TEAM": fromSecret: ":xHoneycombTeam"
			}
		}
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"
	initializeBefore: ["namespacelabs.dev/foundation/std/monitoring/tracing"]

	instantiate: {
		honeycombTeam: secrets.#Exports.Secret & {
			name:     "x-honeycomb-team"
			optional: true // XXX this is temporary until we figure out the testing story.
		}
		openTelemetry: tracing.#Exports.Exporter & {
			name: "honeycomb"
		}
	}
}

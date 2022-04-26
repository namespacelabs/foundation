import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"
	initializeBefore: ["namespacelabs.dev/foundation/std/monitoring/tracing"]

	instantiate: {
		honeycombTeam: secrets.#Exports.Secret & {
			name: "x-honeycomb-team"
		}
		openTelemetry: tracing.#Exports.Exporter & {
			name: "honeycomb"
		}
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

$typesProto: inputs.#Proto & {
	sources: [
		"wire.proto",
	]
}

extension: fn.#Extension & {
	instantiate: {
		readinessCheck: core.#Exports.ReadinessCheck
		openTelemetry:  tracing.#Exports.TracerProvider
	}

	provides: {
		WireDatabase: {
			input: $typesProto.types.WireDatabaseArgs

			availableIn: {
				go: type: "WireDatabase"
			}
		}
	}
}
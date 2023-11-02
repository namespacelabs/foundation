import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	provides: {
		Tracer: {
			input: $typesProto.types.NoArgs
			availableIn: {
				go: {
					package: "go.opentelemetry.io/otel/trace"
					type:    "Tracer"
				}
			}
		}

		Meter: {
			input: $typesProto.types.NoArgs
			availableIn: {
				go: {
					package: "go.opentelemetry.io/otel/metric"
					type:    "Meter"
				}
			}
		}
	}

	instantiate: {
		tracerProvider: tracing.#Exports.TracerProvider
		meterProvider:  tracing.#Exports.MeterProvider
	}
}

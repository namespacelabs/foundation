import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
	"namespacelabs.dev/foundation/std/core/info"
)

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	provides: {
		Exporter: {
			input: $typesProto.types.ExporterArgs
			availableIn: {
				go: type: "Exporter"
			}
		}
		Detector: {
			input: $typesProto.types.DetectorArgs
			availableIn: {
				go: type: "Detector"
			}
		}
		TracerProvider: {
			input: $typesProto.types.NoArgs
			availableIn: {
				go: {
					type: "DeferredTracerProvider"
				}
			}
		}
		MeterProvider: {
			input: $typesProto.types.NoArgs
			availableIn: {
				go: {
					package: "go.opentelemetry.io/otel/sdk/metric"
					type:    "*MeterProvider"
				}
			}
		}

		HttpClientProvider: {
			input: $typesProto.types.NoArgs
			availableIn: {
				go: {
					type: "HttpClientProvider"
				}
			}
		}
	}

	instantiate: {
		serverInfo: info.#Exports.ServerInfo
		"interceptors": interceptors.#Exports.InterceptorRegistration & {
			name: "otel-tracing"
		}
		"middleware": middleware.#Exports.Middleware
	}
}

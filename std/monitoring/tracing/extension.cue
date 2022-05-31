import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
	"namespacelabs.dev/foundation/std/go/core"
)

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"

	provides: {
		Exporter: {
			input: $typesProto.types.ExporterArgs
			availableIn: {
				go: type: "Exporter"
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
		HttpClientProvider: {
			input: $typesProto.types.NoArgs
			availableIn: {
				go: {
					type: "HttpClientProvider"
				}
				nodejs: {
					import: "http"
					type: "HttpClientProvider"
				}
			}
		}
	}

	instantiate: {
		serverInfo:     core.#Exports.ServerInfo
		"interceptors": interceptors.#Exports.InterceptorRegistration
		"middleware":   middleware.#Exports.Middleware
	}
}

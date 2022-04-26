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
	provides: {
		TraceProvider: {
			input: $typesProto.types.TraceProviderArgs
			availableIn: {
				go: type: "TraceProvider"
			}
		}
	}

	instantiate: {
		serverInfo:     core.#Exports.ServerInfo
		"interceptors": interceptors.#Exports.InterceptorRegistration
		"middleware":   middleware.#Exports.Middleware
	}
}

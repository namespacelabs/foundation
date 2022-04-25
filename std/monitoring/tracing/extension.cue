import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"

	instantiate: {
		"interceptors": interceptors.#Exports.InterceptorRegistration
		"middleware":   middleware.#Exports.Middleware
	}
}

$env:          inputs.#Environment
$jaegerServer: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/monitoring/jaeger"
}

configure: fn.#Configure & {
	if $env.runtime == "kubernetes" {
		stack: {
			append: [$jaegerServer]
		}

		startup: {
			args: {
				jaeger_collector_endpoint: "http://\($jaegerServer.$addressMap.collector)/api/traces"
			}
		}
	}
}

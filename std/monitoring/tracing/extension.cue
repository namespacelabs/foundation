import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

extension: fn.#Extension & {
	initializeIn: ["GO"]

	instantiate: {
		"interceptors": interceptors.#Exports.InterceptorRegistration
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

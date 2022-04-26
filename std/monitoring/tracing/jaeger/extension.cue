import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"

	instantiate: {
		openTelemetry: tracing.#Exports.TraceProvider & {
            name: "jaeger"
        }
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

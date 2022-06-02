import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/nodejs/monitoring/tracing"
)

// Registers Jaeger as an Open Telemetry exporter.
// TODO: implement a language-agnostic in-cluster Otel collector,
// and integrate Jaeger with that.

extension: fn.#Extension & {
	hasInitializerIn: "NODEJS"
	initializeBefore: ["namespacelabs.dev/foundation/std/nodejs/monitoring/tracing"]

	instantiate: {
		openTelemetry: tracing.#Exports.Exporter & {
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

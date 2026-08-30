import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"
	initializeBefore: ["namespacelabs.dev/foundation/std/monitoring/tracing"]

	instantiate: {
		openTelemetry: tracing.#Exports.Exporter & {
			name: "jaeger"
		}
	}
}

$jaegerServer: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/monitoring/jaeger"
}

configure: fn.#Configure & {
	stack: {
		append: [$jaegerServer]
	}

	startup: {
		if $jaegerServer.$addressMap.collector != _|_ {
			args: {
				jaeger_collector_endpoint: "http://\($jaegerServer.$addressMap.collector)/api/traces"
			}
		}
	}
}

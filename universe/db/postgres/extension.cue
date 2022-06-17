import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

$typesProto: inputs.#Proto & {
	sources: [
		"database.proto",
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
				nodejs: {
					import: "api"
					type: "WireDatabase"
				}
			}
		}
	}
}

configure: fn.#Configure & {
	init: setup: {
		binary: "namespacelabs.dev/foundation/universe/db/postgres/init"
	}
}

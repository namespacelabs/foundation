import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

// Provides Open Telemetry integration.

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	hasInitializerIn: "NODEJS"

	provides: {
		Exporter: {
			input: $typesProto.types.ExporterArgs
			availableIn: {
				nodejs: {
					import: "exporter"
					type:   "Exporter"
				}
			}
		}
	}
}

import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$coreTypesProto: inputs.#Proto & {
	source: "types/coretypes.proto"
}

extension: fn.#Extension & {
	provides: {
		LivenessCheck: {
			input: $coreTypesProto.types.LivenessCheckArgs
			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/std/go/core"
					type:    "Check"
				}
			}
		}

		ReadinessCheck: {
			input: $coreTypesProto.types.ReadinessCheckArgs
			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/std/go/core"
					type:    "Check"
				}
			}
		}

		DebugHandler: {
			input: $coreTypesProto.types.DebugHandlerArgs
			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/std/go/core"
					type:    "DebugHandler"
				}
			}
		}
	}
}

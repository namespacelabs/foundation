import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$coreTypesProto: inputs.#Proto & {
	source: "../types/coretypes.proto"
}

// We use a separate extension for ServerInfo as VCS causes servers to redeploy on edits
extension: fn.#Extension & {
	provides: {
		ServerInfo: {
			input: $coreTypesProto.types.ServerInfoArgs
			availableIn: {
				go: {
					package: "namespacelabs.dev/foundation/std/core/types"
					type:    "*ServerInfo"
				}
			}
		}
	}
}

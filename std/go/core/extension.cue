import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$coreTypesProto: inputs.#Proto & {
	source: "coretypes.proto"
}

extension: fn.#Extension & {
	provides: {
		LivenessCheck: {
			input: $coreTypesProto.types.LivenessCheckArgs
			availableIn: {
				go: type: "Check"
			}
		}

		ReadinessCheck: {
			input: $coreTypesProto.types.ReadinessCheckArgs
			availableIn: {
				go: type: "Check"
			}
		}

		DebugHandler: {
			input: $coreTypesProto.types.DebugHandlerArgs
			availableIn: {
				go: type: "DebugHandler"
			}
		}

		ServerInfo: {
			input: $coreTypesProto.types.ServerInfoArgs
			availableIn: {
				go: type: "*ServerInfo"
			}
		}
	}
}

$inputs: {
	env:   inputs.#Environment
	vcs:   inputs.#VCS
	focus: inputs.#FocusServer
}

configure: fn.#Configure & {
	startup: {
		args: {
			env_json:      json.Marshal($inputs.env)
			vcs_json:      json.Marshal($inputs.vcs)
			image_version: $inputs.focus.image
		}
	}
}

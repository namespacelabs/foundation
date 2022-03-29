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
			input: $coreTypesProto.types.LivenessCheck
			availableIn: {
				go: type: "Check"
			}
		}

		ReadinessCheck: {
			input: $coreTypesProto.types.ReadinessCheck
			availableIn: {
				go: type: "Check"
			}
		}
	}
}

$inputs: {
	env:   inputs.#Environment
	focus: inputs.#FocusServer
}

configure: fn.#Configure & {
	startup: {
		args: {
			env_json:      json.Marshal($inputs.env)
			image_version: $inputs.focus.image
		}
	}
}

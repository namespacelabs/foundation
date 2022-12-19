import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {
	source: "provider.proto"
}

extension: fn.#Extension & {
	resources: {
		"root-user": {
			kind:  "namespacelabs.dev/foundation/library/runtime:Secret"
			input: ":root-user"
		}
		"root-password": {
			kind:  "namespacelabs.dev/foundation/library/runtime:Secret"
			input: ":root-password"
		}
	}

	provides: {
		Creds: {
			input: $providerProto.types.CredsRequest

			availableIn: {
				go: type: "*Creds"
			}
		}
	}
}

secrets: {
	"root-user": {
		description: "MinIO root user."
		generate: {
			uniqueId:        "root-user"
			randomByteCount: 8
			format:          "FORMAT_BASE32"
		}
	}
	"root-password": {
		description: "MinIO root user password."
		generate: {
			uniqueId:        "root-password"
			randomByteCount: 16
			format:          "FORMAT_BASE32"
		}
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	resources: {
		"token-secret-resource": {
			kind:  "namespacelabs.dev/foundation/library/runtime:Secret"
			input: ":token-secret"
		}
	}
}

secrets: {
	"token-secret": {
		description: "A generated secret, for testing purposes."
		generate: {
			uniqueId:        "http-csrf-token"
			randomByteCount: 32
			format:          "FORMAT_BASE64"
		}
	}
}

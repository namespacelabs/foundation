import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/secrets"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"

	instantiate: token: secrets.#Exports.Secret & {
		name: "http_csrf_token"
		generate: {
			randomByteCount: 32
			format:          "FORMAT_BASE64"
		}
	}
}

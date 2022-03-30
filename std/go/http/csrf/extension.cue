import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/secrets"
)

extension: fn.#Extension & {
	initializeIn: ["GO"]

	instantiate: token: secrets.#Exports.Secret & {
		with: {
			name: "http_csrf_token"
			provision: ["PROVISION_INLINE"]
		}
	}
}

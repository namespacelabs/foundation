import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/secrets"
)

extension: fn.#Extension & {
	instantiate: {
		tsKey: secrets.#Exports.Secret & {
			name: "tailscale-auth-key"
		}
	}
}

configure: fn.#Configure & {
	sidecar: tailscaled: {
		binary: "namespacelabs.dev/foundation/universe/networking/tailscale/image"
	}
}

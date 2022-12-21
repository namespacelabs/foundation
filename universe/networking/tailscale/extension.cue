import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
}

configure: fn.#Configure & {
	sidecar: tailscaled: {
		binary: "namespacelabs.dev/foundation/universe/networking/tailscale/image"
	}
	startup: {
		env: {
			"TAILSCALE_AUTH_KEY": fromSecret: ":tsAuthKey"
		}
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
)

extension: fn.#Extension & {
	packageData: [
		"defaults/container.securitycontext.yaml",
		"defaults/pod.podsecuritycontext.yaml",
	]
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/std/grpc"
)

service: fn.#Service & {
	framework: "GO"

	// We don't really need it, but https://github.com/namespacelabs/foundation/issues/717
	instantiate: {
		ready: core.#Exports.ReadinessCheck

		orchestrator: grpc.#Exports.Backend & {
			packageName: "namespacelabs.dev/foundation/orchestration/service"
		}
	}
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/orchestration/controllers/crds/revision/configure"
}

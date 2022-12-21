import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "0fomj22adbua2u0ug3og"
	name:      "orchestration-api-server"
	framework: "GO"

	import: [
		"namespacelabs.dev/foundation/orchestration/controllers/crds/revision",
		"namespacelabs.dev/foundation/orchestration/controllers/runtimeconfig",
		"namespacelabs.dev/foundation/orchestration/service",
		"namespacelabs.dev/foundation/orchestration/legacycontroller", // TODO remove
		"namespacelabs.dev/foundation/std/grpc/logging",
	]
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/orchestration/server/tool"
}

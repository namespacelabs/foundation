import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:        "0fomj22adbua2u0ug3og"
	name:      "orchestration-api-server"
	framework: "GO"

	resourceRequests: {
		cpu:    "100m"
		memory: "128Mi"
	}

	import: [
		"namespacelabs.dev/foundation/orchestration/service",
		"namespacelabs.dev/foundation/std/grpc/logging",
	]
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/orchestration/server/tool"
}

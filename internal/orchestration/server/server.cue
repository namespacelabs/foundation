import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:           "0fomj22adbua2u0ug3og"
	name:         "orchestration-api-server"
	clusterAdmin: true
	isStateful:   true

	binary: "namespacelabs.dev/foundation/internal/orchestration/server/img"

	import: [
		"namespacelabs.dev/foundation/internal/orchestration/server/data",
	]

	service: "orchestration-service": {
		containerPort: 40962
		metadata: protocol: "tcp"
	}
}

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/internal/orchestration/server/tool"
	startup: {
		args: {
			listen_hostname: "0.0.0.0"
			port:            "40962"
		}
	}
}

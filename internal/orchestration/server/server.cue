import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#Server & {
	id:           "0fomj22adbua2u0ug3og"
	name:         "orchestration-api-server"
	framework:    "GO"
	clusterAdmin: true

	import: [
		"namespacelabs.dev/foundation/internal/orchestration/service",
	]
}

import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "fdsq5k9jc3k5s3onhv3g"
	name: "redis-server"

	isStateful: true

	binary: image: "redis:6.2.6-alpine@sha256:132337b9d7744ffee4fae83f51de53c3530935ad3ba528b7110f2d805f55cbf5"

	service: "redis": {
		containerPort: 6379
		metadata: protocol: "tcp"
	}

	import: [
		"namespacelabs.dev/foundation/universe/db/redis/server/datastorage",
	]
}

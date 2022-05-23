import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "iak0a1srli3s1v4eb08g"
	name: "minio"

	isStateful: true

	binary: image: "minio/minio"

	import: [
		"namespacelabs.dev/foundation/universe/storage/minio/configure",
	]

	service: "api": {
		containerPort: 9000
		metadata: {
			protocol: "http"
		}
	}

	service: "console": {
		containerPort: 9001
		metadata: {
			protocol: "http"
		}
	}
}

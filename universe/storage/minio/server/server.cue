import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "iak0a1srli3s1v4eb08g"
	name: "minio"

	isStateful: true

	binary: image: "minio/minio@sha256:de46799fc1ced82b784554ba4602b677a71966148b77f5028132fc50adf37b1f"

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

import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "iak0a1srli3s1v4eb08g"
	name: "minio"

	isStateful: true

	binary: image: "minio/minio@sha256:de46799fc1ced82b784554ba4602b677a71966148b77f5028132fc50adf37b1f"

	env: {
		MINIO_ROOT_USER: fromSecret:     "namespacelabs.dev/foundation/universe/storage/minio/creds:root-user"
		MINIO_ROOT_PASSWORD: fromSecret: "namespacelabs.dev/foundation/universe/storage/minio/creds:root-password"
	}

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

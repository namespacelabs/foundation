import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "iak0a1srli3s1v4eb08g"
	name: "minio"

	isStateful: true

	binary: image: "quay.io/minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e"

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

import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "iak0a1srli3s1v4eb08g"
	name: "minio"

	isStateful: true

	binary: image: "public.registry.namespace.systems/minio/minio:RELEASE.2025-06-13T11-33-47Z@sha256:6bec3986282f95cdf90dedf776bbbed2d84f2a2a8c234cda369fe1036f31960a"

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

server: {
	name: "minio-server"

	image: "public.registry.namespace.systems/minio/minio:RELEASE.2025-06-13T11-33-47Z@sha256:6bec3986282f95cdf90dedf776bbbed2d84f2a2a8c234cda369fe1036f31960a"

	// MinIO acts as an object store which requires a stateful deployment (more conservative update strategy).
	class: "stateful"

	env: {
		// Disable update checking as self-update will never be used.
		MINIO_UPDATE: "off"

		MINIO_ROOT_USER: fromSecret:     ":user"
		MINIO_ROOT_PASSWORD: fromSecret: ":password"
	}

	args: [
		"server",
		"/minio",
		"--address=:9000",
		"--console-address=:9001",
	]

	mounts: {
		"/minio": persistent: {
			// Unique volume identifier
			id:       "minio-server-data"
			size:     "10GiB"
			template: true
		}
	}
}

secrets: {
	"user": {
		description: "Minio root user"
		generate: {
			uniqueId:        "minio-user"
			randomByteCount: 32
			format:          "FORMAT_BASE32"
		}
	}
	"password": {
		description: "Minio root password"
		generate: {
			uniqueId:        "minio-password"
			randomByteCount: 32
			format:          "FORMAT_BASE32"
		}
	}
}

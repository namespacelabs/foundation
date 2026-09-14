server: {
	name: "minio-server"

	image: "quay.io/minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e"

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

server: {
	name: "minio-server"

	image: "minio/minio@sha256:de46799fc1ced82b784554ba4602b677a71966148b77f5028132fc50adf37b1f"

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

	services: {
		api: {
			port: 9000
			kind: "http"
			probe: http: "/minio/health/ready"
		}
		console: {
			port: 9001
			kind: "http"
		}
	}

	mounts: {
		"/minio": persistent: {
			// Unique volume identifier
			id:   "minio-server-data"
			size: "10GiB"
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

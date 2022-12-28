server: {
	name: "redis-server"

	image: "redis:6.2.6-alpine@sha256:132337b9d7744ffee4fae83f51de53c3530935ad3ba528b7110f2d805f55cbf5"

	class: "stateful"

	services: "redis": {
		port: 6379
		kind: "tcp"
	}

	args: [
		// Dump the dataset to disk every 60 seconds if any key changed.
		// https://redis.io/docs/management/persistence/#snapshotting
		"--save", "60", "1",

		// Password value injected from environment variable by Kubernetes.
		// https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#use-environment-variables-to-define-arguments
		"--requirepass", "$(REDIS_ROOT_PASSWORD)",
	]

	env: {
		REDIS_ROOT_PASSWORD: fromSecret: ":password"
	}

	probe: exec: ["redis-cli", "ping"]

	mounts: {
		"/data": ":data"
	}
}

volumes: {
	"data": persistent: {
		id:   "redis-data"
		size: "10GiB"
	}
}

secrets: {
	"password": {
		description: "Redis root password"
		generate: {
			uniqueId:        "redis-password"
			randomByteCount: 32
			format:          "FORMAT_BASE32"
		}
	}
}

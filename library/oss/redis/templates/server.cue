package templates

#Server: {
	spec: {
		image:          *"redis:6.2.6-alpine@sha256:132337b9d7744ffee4fae83f51de53c3530935ad3ba528b7110f2d805f55cbf5" | string
		dataVolumeSize: *"10GiB" | string
		dataVolume:     *{
			id:   "redis-server-data"
			size: dataVolumeSize
		} | {
			id:   string
			size: string
		}
		passwordSecret:   *"namespacelabs.dev/foundation/library/oss/redis/server:password" | string
		snapshotInterval: *"60" | string
	}
	name: "redis-server"

	image: spec.image

	class: "stateful"

	services: "redis": {
		port: 6379
		kind: "tcp"
	}

	args: [
		// Dump the dataset to disk every 60 seconds if any key changed.
		// https://redis.io/docs/management/persistence/#snapshotting
		"--save", "$(REDIS_SNAPSHOT_INTERVAL)", "1",

		// Password value injected from environment variable by Kubernetes.
		// https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#use-environment-variables-to-define-arguments
		"--requirepass", "$(REDIS_ROOT_PASSWORD)",
	]

	env: {
		REDIS_ROOT_PASSWORD: fromSecret: spec.passwordSecret
		REDIS_SNAPSHOT_INTERVAL: spec.snapshotInterval
	}

	probe: exec: ["redis-cli", "ping"]

	mounts: {
		"/data": persistent: spec.dataVolume
	}
}

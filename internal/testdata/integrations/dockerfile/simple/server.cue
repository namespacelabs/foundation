server: {
	name: "mysimpleserver"

	args: ["start"]

	integration: dockerfile: {
		command: "npm"
	}

	env: {
		REDIS_DB: fromResourceField: {
			resource: ":cache"
			fieldRef: "database"
		}

	}

	resourceLimits: {
		memory: "256Mi"
		cpu:    "500m"
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: provider: "namespacelabs.dev/foundation/library/kubernetes/ingress"
		}
	}

	mounts: "/app/src": {
		syncWorkspace: fromDir: "src"
	}

	resources: {
		cache: {
			class:    "namespacelabs.dev/foundation/library/database/redis:Database"
			provider: "namespacelabs.dev/foundation/library/oss/redis"

			intent: {
				database: 1
			}
		}
	}
}

providers: {
	"namespacelabs.dev/foundation/library/database/redis:Database": {
		initializedWith: imageFrom: binary: "namespacelabs.dev/foundation/library/oss/redis/prepare"

		resources: {
			// Adds the server to the stack
			redisServer: {
				class: "namespacelabs.dev/foundation/library/runtime:Server"
				intent: package_name: "namespacelabs.dev/foundation/library/oss/redis/server"
			}
		}
	}
}

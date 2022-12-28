providers: {
	"namespacelabs.dev/foundation/library/database/redis:Database": {
		initializedWith: "namespacelabs.dev/foundation/library/oss/redis/prepare"

		intent: {
			type:   "library.oss.redis.DatabaseIntent"
			source: "./types.proto"
		}

		resources: {
			// Adds the server to the stack
			redisServer: {
				class:  "namespacelabs.dev/foundation/library/runtime:Server"
				intent: "namespacelabs.dev/foundation/library/oss/redis/server"
			}
			// Mounts the Redis root password as a secret
			password: {
				class:  "namespacelabs.dev/foundation/library/runtime:Secret"
				intent: "namespacelabs.dev/foundation/library/oss/redis/server:password"
			}
		}
	}
}

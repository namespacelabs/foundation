providers: {
	"namespacelabs.dev/foundation/library/database/postgres:Database": {
		initializedWith: imageFrom: binary: "namespacelabs.dev/foundation/library/oss/postgres/prepare"

		resources: {
			// Adds the server to the stack
			postgresServer: {
				class: "namespacelabs.dev/foundation/library/runtime:Server"
				intent: package_name: "namespacelabs.dev/foundation/library/oss/postgres/server"
			}
			// Mounts the Postgres password as a secret
			postgresPassword: {
				class: "namespacelabs.dev/foundation/library/runtime:Secret"
				intent: ref: "namespacelabs.dev/foundation/library/oss/postgres/server:password"
			}
		}
	}
}

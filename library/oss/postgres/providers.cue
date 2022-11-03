providers: {
	"namespacelabs.dev/foundation/library/database/postgres:Database": {
		initializedWith: imageFrom: binary: "namespacelabs.dev/foundation/library/oss/postgres/prepare/database"

		// Requires namespacelabs.dev/foundation/library/oss/postgres:cluster to be provided.
	}
	"namespacelabs.dev/foundation/library/database/postgres:Cluster": {
		initializedWith: imageFrom: binary: "namespacelabs.dev/foundation/library/oss/postgres/prepare/cluster"

		resources: {
			// Adds the server to the stack
			server: {
				class: "namespacelabs.dev/foundation/library/runtime:Server"
				intent: package_name: "namespacelabs.dev/foundation/library/oss/postgres/server"
			}
			// Mounts the Postgres password as a secret
			password: {
				class: "namespacelabs.dev/foundation/library/runtime:Secret"
				intent: ref: "namespacelabs.dev/foundation/library/oss/postgres/server:password"
			}
		}
	}
}

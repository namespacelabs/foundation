import "namespacelabs.dev/foundation/library/oss/postgres/templates"

server: templates.#Server

secrets: {
	"password": {
		description: "Postgres server password"
		generate: {
			uniqueId:        "postgres-password"
			randomByteCount: 32
			format:          "FORMAT_BASE32"
		}
	}
}

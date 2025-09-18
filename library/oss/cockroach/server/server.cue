import "namespacelabs.dev/foundation/library/oss/cockroach/templates"

server: templates.#Server

secrets: {
	"password": {
		description: "CockroachDB server password"
		generate: {
			uniqueId:        "cockroach-password"
			randomByteCount: 32
			format:          "FORMAT_BASE32"
		}
	}
}

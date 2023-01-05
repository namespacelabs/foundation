import "namespacelabs.dev/foundation/library/oss/redis/templates"

server: templates.#Server

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

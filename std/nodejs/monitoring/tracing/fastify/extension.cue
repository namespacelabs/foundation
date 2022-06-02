import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/nodejs/http"
)

// Registers Fastify as an Open Telemetry data source.

extension: fn.#Extension & {
	hasInitializerIn: "NODEJS"

	instantiate: {
		httpServer: http.#Exports.HttpServer
	}
}

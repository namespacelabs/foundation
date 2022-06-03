import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/nodejs/http"
)

// Registers Fastify as an Open Telemetry data source.

extension: fn.#Extension & {
	hasInitializerIn: "NODEJS"
	initializeAfter: ["namespacelabs.dev/foundation/std/nodejs/monitoring/tracing"]

	instantiate: {
		httpServer: http.#Exports.HttpServer
	}
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/core/info"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
)

// Using this extension requires the environment variable MONITORING_SENTRY_DSN
// to be set; this is a work-around until secrets can be passed in.
extension: fn.#Extension & {
	hasInitializerIn: "GO"

	instantiate: {
		serverInfo:     info.#Exports.ServerInfo
		"interceptors": interceptors.#Exports.InterceptorRegistration
		"middleware":   middleware.#Exports.Middleware
	}
}

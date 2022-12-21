import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/core/info"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	instantiate: {
		serverInfo:     info.#Exports.ServerInfo
		"interceptors": interceptors.#Exports.InterceptorRegistration
		"middleware":   middleware.#Exports.Middleware
	}
}

configure: fn.#Configure & {
	startup: {
		env: {
			"MONITORING_SENTRY_DSN": fromSecret: ":sentryDsn"
		}
	}
}

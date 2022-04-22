import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"

	instantiate: {
		dsn: secrets.#Exports.Secret & {
			name: "sentry-dsn"
		}
		serverInfo:     core.#Exports.ServerInfo
		"interceptors": interceptors.#Exports.InterceptorRegistration
	}
}

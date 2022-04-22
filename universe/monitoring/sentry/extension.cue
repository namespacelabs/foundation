import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/go/core"
)

extension: fn.#Extension & {
    hasInitializerIn: "GO_GRPC"

	instantiate: {
		dsn: secrets.#Exports.Secret & {
			name: "sentry-dsn"
		}
        serverInfo: core.#Exports.ServerInfo
	}
}
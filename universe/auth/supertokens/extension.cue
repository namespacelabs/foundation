import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/go/http/middleware"
	"namespacelabs.dev/foundation/std/secrets"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	instantiate: {
		"middleware":   middleware.#Exports.Middleware
		githubClientId: secrets.#Exports.Secret & {
			name: "github_client_id"
		}
		githubClientSecret: secrets.#Exports.Secret & {
			name: "github_client_secret"
		}
	}
}

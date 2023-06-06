import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	instantiate: {
		"interceptors": interceptors.#Exports.InterceptorRegistration & {
			name: "grpc-logging"
			after: ["namespace-rid"]
		}
		"middleware": middleware.#Exports.Middleware
	}
}

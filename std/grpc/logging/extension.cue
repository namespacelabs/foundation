import (
	"encoding/json"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO"

	instantiate: {
		"interceptors": interceptors.#Exports.InterceptorRegistration
	}
}

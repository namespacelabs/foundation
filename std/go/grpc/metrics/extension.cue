import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

extension: fn.#Extension & {
	hasInitializerIn: "GO_GRPC"

	instantiate: {
		"interceptors": interceptors.#Exports.InterceptorRegistration
	}

	import: [
		"namespacelabs.dev/foundation/std/monitoring/prometheus",
	]
}

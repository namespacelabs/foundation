import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

extension: fn.#Extension & {
	initializeIn: ["GO"]

	instantiate: {
		"interceptors": interceptors.#Exports.InterceptorRegistration
	}

	import: [
		"namespacelabs.dev/foundation/std/monitoring/prometheus",
	]
}

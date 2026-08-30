import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	instantiate: {
		openTelemetry: tracing.#Exports.TracerProvider
	}

	provides: {
		ClientFactory: {
			input: $typesProto.types.ClientFactoryArgs
			availableIn: {
				go: {
					type: "ClientFactory"
				}
			}
		}
	}

	// Enable IAM Role / Service Account translation by default.
	import: [
		"namespacelabs.dev/foundation/universe/aws/irsa",
	]
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	instantiate: {
		"credentials": secrets.#Exports.Secret & {
			name: "aws_credentials_file"
			optional: true
		}
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
}

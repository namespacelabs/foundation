import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/nodejs/grpc"
	"namespacelabs.dev/foundation/languages/nodejs/testdata/extensions/batchformatter"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

// A service that uses the "batchformatter" extension.

service: fn.#Service & {
	framework: "NODEJS"

	instantiate: {
		grpcRegistrar: grpc.#Exports.GrpcRegistrar
		// Instantiating the "batchformatter" extension twice to show the scopes of its dependencies.
		batch1: batchformatter.#Exports.BatchFormatter
		batch2: batchformatter.#Exports.BatchFormatter
	}

	exportService: $proto.services.FormatService
}

import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/testdata/scopes"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
	framework: "GO_GRPC"

	instantiate: {
		one: scopes.#Exports.ScopedData
		two: scopes.#Exports.ScopedData
	}

	exportService: $proto.services.ModelingService
}

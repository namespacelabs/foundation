import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/nodejs/grpc"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
	framework: "NODEJS"

	instantiate: {
		grpcRegistrar: grpc.#Exports.GrpcRegistrar
	}

	exportService: $proto.services.PostService
}

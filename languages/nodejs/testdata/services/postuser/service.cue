import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/grpc"
	nodejsgrpc "namespacelabs.dev/foundation/std/nodejs/grpc"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

// Example of a service that uses another service: PostService in "services/simple".

service: fn.#Service & {
	framework: "NODEJS"

	instantiate: {
		grpcRegistrar: nodejsgrpc.#Exports.GrpcRegistrar
		postService:   grpc.#Exports.Backend & {
			packageName: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple"
		}
	}

	exportService: $proto.services.PostUserService
}

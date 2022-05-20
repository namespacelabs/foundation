import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/testdata/datastore"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
)

$proto: inputs.#Proto & {
	source: "../proto/simple.proto"
}

service: fn.#Service & {
	framework:     "GO_GRPC"
	exportService: $proto.services.EmptyService
}

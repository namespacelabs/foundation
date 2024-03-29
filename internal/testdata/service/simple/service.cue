import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/internal/testdata/datastore"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
)

$proto: inputs.#Proto & {
	source: "../proto/simple.proto"
}

service: fn.#Service & {
	framework:     "GO"
	exportService: $proto.services.EmptyService
}

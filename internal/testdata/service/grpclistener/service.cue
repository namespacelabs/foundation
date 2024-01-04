import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$proto: inputs.#Proto & {
	source: "../proto/simple.proto"
}

service: fn.#Service & {
	framework:     "GO"
	exportService: $proto.services.EmptyService
	listener:      "mtls"
	ingress:       "INTERNET_FACING"
}

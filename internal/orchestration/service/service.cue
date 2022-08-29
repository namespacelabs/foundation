import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$proto: inputs.#Proto & {
	source: "./proto/service.proto"
}

service: fn.#Service & {
	framework:     "GO"
	exportService: $proto.services.OrchestrationService
}

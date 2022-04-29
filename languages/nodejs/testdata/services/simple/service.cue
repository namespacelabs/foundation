import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
  framework: "NODEJS"

	exportService:        $proto.services.PostService
}
